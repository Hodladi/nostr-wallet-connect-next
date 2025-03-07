import { BackendType } from "src/types";

type BackendTypeConfig = {
  hasMnemonic: boolean;
  hasChannelManagement: boolean;
  hasNodeBackup: boolean;
};

export const backendTypeConfigs: Record<BackendType, BackendTypeConfig> = {
  LND: {
    hasMnemonic: false,
    hasChannelManagement: false, // TODO: set to true soon
    hasNodeBackup: false,
  },
  BREEZ: {
    hasMnemonic: true,
    hasChannelManagement: false,
    hasNodeBackup: false,
  },
  GREENLIGHT: {
    hasMnemonic: true,
    hasChannelManagement: true,
    hasNodeBackup: false,
  },
  LDK: {
    hasMnemonic: true,
    hasChannelManagement: true,
    hasNodeBackup: true,
  },
  PHOENIX: {
    hasMnemonic: false,
    hasChannelManagement: false,
    hasNodeBackup: false,
  },
  CASHU: {
    hasMnemonic: false,
    hasChannelManagement: false,
    hasNodeBackup: false,
  },
};
