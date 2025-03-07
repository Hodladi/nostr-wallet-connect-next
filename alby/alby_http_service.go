package alby

import (
	"fmt"
	"net/http"

	"github.com/getAlby/nostr-wallet-connect/config"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type AlbyHttpService struct {
	albyOAuthSvc AlbyOAuthService
	logger       *logrus.Logger
	appConfig    *config.AppConfig
}

func NewAlbyHttpService(albyOAuthSvc AlbyOAuthService, logger *logrus.Logger, appConfig *config.AppConfig) *AlbyHttpService {
	return &AlbyHttpService{
		albyOAuthSvc: albyOAuthSvc,
		logger:       logger,
		appConfig:    appConfig,
	}
}

func (albyHttpSvc *AlbyHttpService) RegisterSharedRoutes(e *echo.Echo, authMiddleware func(next echo.HandlerFunc) echo.HandlerFunc) {
	e.GET("/api/alby/callback", albyHttpSvc.albyCallbackHandler, authMiddleware)
	e.GET("/api/alby/me", albyHttpSvc.albyMeHandler, authMiddleware)
	e.GET("/api/alby/balance", albyHttpSvc.albyBalanceHandler, authMiddleware)
	e.POST("/api/alby/pay", albyHttpSvc.albyPayHandler, authMiddleware)
	e.POST("/api/alby/link-account", albyHttpSvc.albyLinkAccountHandler, authMiddleware)
}

func (albyHttpSvc *AlbyHttpService) albyCallbackHandler(c echo.Context) error {
	code := c.QueryParam("code")

	err := albyHttpSvc.albyOAuthSvc.CallbackHandler(c.Request().Context(), code)
	if err != nil {
		albyHttpSvc.logger.WithError(err).Error("Failed to handle Alby OAuth callback")
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: fmt.Sprintf("Failed to handle Alby OAuth callback: %s", err.Error()),
		})
	}

	if albyHttpSvc.appConfig.IsDefaultClientId() {
		// do not redirect if using default OAuth client
		// redirect will be handled by the frontend instead
		return c.NoContent(http.StatusNoContent)
	}

	redirectUrl := albyHttpSvc.appConfig.FrontendUrl
	if redirectUrl == "" {
		redirectUrl = albyHttpSvc.appConfig.BaseUrl
	}

	return c.Redirect(http.StatusFound, redirectUrl)
}

func (albyHttpSvc *AlbyHttpService) albyMeHandler(c echo.Context) error {
	me, err := albyHttpSvc.albyOAuthSvc.GetMe(c.Request().Context())
	if err != nil {
		albyHttpSvc.logger.WithError(err).Error("Failed to request alby me endpoint")
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: fmt.Sprintf("Failed to request alby me endpoint: %s", err.Error()),
		})
	}

	return c.JSON(http.StatusOK, me)
}

func (albyHttpSvc *AlbyHttpService) albyBalanceHandler(c echo.Context) error {
	balance, err := albyHttpSvc.albyOAuthSvc.GetBalance(c.Request().Context())
	if err != nil {
		albyHttpSvc.logger.WithError(err).Error("Failed to request alby balance endpoint")
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: fmt.Sprintf("Failed to request alby balance endpoint: %s", err.Error()),
		})
	}

	return c.JSON(http.StatusOK, &AlbyBalanceResponse{
		Sats: balance.Balance,
	})
}

func (albyHttpSvc *AlbyHttpService) albyPayHandler(c echo.Context) error {
	var payRequest AlbyPayRequest
	if err := c.Bind(&payRequest); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: fmt.Sprintf("Bad request: %s", err.Error()),
		})
	}

	err := albyHttpSvc.albyOAuthSvc.SendPayment(c.Request().Context(), payRequest.Invoice)
	if err != nil {
		albyHttpSvc.logger.WithError(err).Error("Failed to request alby pay endpoint")
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: fmt.Sprintf("Failed to request alby pay endpoint: %s", err.Error()),
		})
	}

	return c.NoContent(http.StatusNoContent)
}

func (albyHttpSvc *AlbyHttpService) albyLinkAccountHandler(c echo.Context) error {
	err := albyHttpSvc.albyOAuthSvc.LinkAccount(c.Request().Context())
	if err != nil {
		albyHttpSvc.logger.WithError(err).Error("Failed to connect alby account")
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
