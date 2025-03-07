package backup

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"archive/zip"
	"os"
	"path/filepath"
	"slices"

	"github.com/getAlby/nostr-wallet-connect/service"
	"github.com/sirupsen/logrus"

	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"

	"golang.org/x/crypto/pbkdf2"
)

type backupService struct {
	svc    service.Service
	logger *logrus.Logger
}

func NewBackupService(svc service.Service, logger *logrus.Logger) *backupService {
	return &backupService{
		svc:    svc,
		logger: logger,
	}
}

func (bs *backupService) CreateBackup(unlockPassword string, w io.Writer) error {
	var err error

	if !bs.svc.GetConfig().CheckUnlockPassword(unlockPassword) {
		return errors.New("invalid unlock password")
	}

	workDir, err := filepath.Abs(bs.svc.GetConfig().GetEnv().Workdir)
	if err != nil {
		return fmt.Errorf("failed to get absolute workdir: %w", err)
	}

	lnStorageDir := ""

	if bs.svc.GetLNClient() == nil {
		return fmt.Errorf("node not running")
	}
	lnStorageDir, err = bs.svc.GetLNClient().GetStorageDir()
	if err != nil {
		return fmt.Errorf("failed to get storage dir: %w", err)
	}
	bs.logger.WithField("path", lnStorageDir).Info("Found node storage dir")

	// Reset the routing data to decrease the LDK DB size
	err = bs.svc.GetLNClient().ResetRouter("ALL")
	if err != nil {
		bs.logger.WithError(err).Error("Failed to reset router")
		return fmt.Errorf("failed to reset router: %w", err)
	}
	// Stop the app to ensure no new requests are processed.
	bs.svc.StopApp()

	// Closing the database leaves the service in an inconsistent state,
	// but that should not be a problem since the app is not expected
	// to be used after its data is exported.
	err = bs.svc.StopDb()
	if err != nil {
		bs.logger.WithError(err).Error("Failed to stop database")
		return fmt.Errorf("failed to close database: %w", err)
	}

	var filesToArchive []string

	if lnStorageDir != "" {
		lnFiles, err := filepath.Glob(filepath.Join(workDir, lnStorageDir, "*"))
		if err != nil {
			return fmt.Errorf("failed to list files in the LNClient storage directory: %w", err)
		}
		bs.logger.WithField("lnFiles", lnFiles).Info("Listed node storage dir")

		// Avoid backing up log files.
		slices.DeleteFunc(lnFiles, func(s string) bool {
			return filepath.Ext(s) == ".log"
		})

		filesToArchive = append(filesToArchive, lnFiles...)
	}

	cw, err := encryptingWriter(w, unlockPassword)
	if err != nil {
		return fmt.Errorf("failed to create encrypted writer: %w", err)
	}

	zw := zip.NewWriter(cw)
	defer zw.Close()

	addFileToZip := func(fsPath, zipPath string) error {
		inF, err := os.Open(fsPath)
		if err != nil {
			return fmt.Errorf("failed to open source file for reading: %w", err)
		}
		defer inF.Close()

		outW, err := zw.Create(zipPath)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		_, err = io.Copy(outW, inF)
		return err
	}

	// Locate the main database file.
	dbFilePath := bs.svc.GetConfig().GetEnv().DatabaseUri
	// Add the database file to the archive.
	bs.logger.WithField("nwc.db", dbFilePath).Info("adding nwc db to zip")
	err = addFileToZip(dbFilePath, "nwc.db")
	if err != nil {
		bs.logger.WithError(err).Error("Failed to zip nwc db")
		return fmt.Errorf("failed to write nwc db file to zip: %w", err)
	}

	for _, fileToArchive := range filesToArchive {
		bs.logger.WithField("fileToArchive", fileToArchive).Info("adding file to zip")
		relPath, err := filepath.Rel(workDir, fileToArchive)
		if err != nil {
			bs.logger.WithError(err).Error("Failed to get relative path of input file")
			return fmt.Errorf("failed to get relative path of input file: %w", err)
		}

		// Ensure forward slashes for zip format compatibility.
		err = addFileToZip(fileToArchive, filepath.ToSlash(relPath))
		if err != nil {
			bs.logger.WithError(err).Error("Failed to write file to zip")
			return fmt.Errorf("failed to write input file to zip: %w", err)
		}
	}

	return nil
}

func (bs *backupService) RestoreBackup(unlockPassword string, r io.Reader) error {
	workDir, err := filepath.Abs(bs.svc.GetConfig().GetEnv().Workdir)
	if err != nil {
		return fmt.Errorf("failed to get absolute workdir: %w", err)
	}

	if strings.HasPrefix(bs.svc.GetConfig().GetEnv().DatabaseUri, "file:") {
		return errors.New("cannot restore backup when database path is a file URI")
	}

	cr, err := decryptingReader(r, unlockPassword)
	if err != nil {
		return fmt.Errorf("failed to create decrypted reader: %w", err)
	}

	tmpF, err := os.CreateTemp("", "nwc-*.bkp")
	if err != nil {
		return fmt.Errorf("failed to create temporary output file: %w", err)
	}
	tmpName := tmpF.Name()
	defer os.Remove(tmpName)
	defer tmpF.Close()

	zipSize, err := io.Copy(tmpF, cr)
	if err != nil {
		return fmt.Errorf("failed to decrypt backup data into temporary file: %w", err)
	}

	if err = tmpF.Sync(); err != nil {
		return fmt.Errorf("failed to flush temporary file: %w", err)
	}

	if _, err = tmpF.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek to beginning of temporary file: %w", err)
	}

	zr, err := zip.NewReader(tmpF, zipSize)
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %w", err)
	}

	extractZipEntry := func(zipFile *zip.File) error {
		fsFilePath := filepath.Join(workDir, "restore", filepath.FromSlash(zipFile.Name))

		if err = os.MkdirAll(filepath.Dir(fsFilePath), 0700); err != nil {
			return fmt.Errorf("failed to create directory for zip entry: %w", err)
		}

		inF, err := zipFile.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry for reading: %w", err)
		}
		defer inF.Close()

		outF, err := os.OpenFile(fsFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed to create destination file: %w", err)
		}
		defer outF.Close()

		if _, err = io.Copy(outF, inF); err != nil {
			return fmt.Errorf("failed to write zip entry to destination file: %w", err)
		}

		return nil
	}

	bs.logger.WithField("count", len(zr.File)).Info("Extracting files")
	for _, f := range zr.File {
		bs.logger.WithField("file", f.Name).Info("Extracting file")
		if err = extractZipEntry(f); err != nil {
			return fmt.Errorf("failed to extract zip entry: %w", err)
		}
	}
	bs.logger.WithField("count", len(zr.File)).Info("Extracted files")

	go func() {
		bs.logger.Info("Backup restored. Shutting down Alby Hub...")
		// schedule node shutdown after a few seconds to ensure frontend updates
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}()

	return nil
}

func encryptingWriter(w io.Writer, password string) (io.Writer, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	encKey := pbkdf2.Key([]byte(password), salt, 4096, 32, sha256.New)
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = rand.Read(iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	_, err = w.Write(salt)
	if err != nil {
		return nil, fmt.Errorf("failed to write salt: %w", err)
	}

	_, err = w.Write(iv)
	if err != nil {
		return nil, fmt.Errorf("failed to write IV: %w", err)
	}

	stream := cipher.NewOFB(block, iv)
	cw := &cipher.StreamWriter{
		S: stream,
		W: w,
	}

	return cw, nil
}

func decryptingReader(r io.Reader, password string) (io.Reader, error) {
	salt := make([]byte, 8)
	if _, err := io.ReadFull(r, salt); err != nil {
		return nil, fmt.Errorf("failed to read salt: %w", err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(r, iv); err != nil {
		return nil, fmt.Errorf("failed to read IV: %w", err)
	}

	encKey := pbkdf2.Key([]byte(password), salt, 4096, 32, sha256.New)
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	stream := cipher.NewOFB(block, iv)
	cr := &cipher.StreamReader{
		S: stream,
		R: r,
	}

	return cr, nil
}
