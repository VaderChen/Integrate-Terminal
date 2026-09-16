import type { Dispatch, SetStateAction } from 'react';
import { extractErrorMessage } from '../appUtils';
import type { Config, PurchaseStatus } from '../types';

type Params = {
  connectionFailed: string;
  setPurchaseStatus: Dispatch<SetStateAction<PurchaseStatus>>;
  setConfig: Dispatch<SetStateAction<Config>>;
  setErrorMessage: Dispatch<SetStateAction<string>>;
};

export function usePurchaseActions({ connectionFailed, setPurchaseStatus, setConfig, setErrorMessage }: Params) {
  const syncPurchaseStatus = (nextStatus: PurchaseStatus) => {
    setPurchaseStatus(nextStatus);
    setConfig((current) => ({ ...current, proUnlock: nextStatus.proUnlock }));
  };

  const runPurchaseAction = async (action: () => Promise<PurchaseStatus> | undefined) => {
    try {
      const nextStatus = await action();
      if (nextStatus) {
        syncPurchaseStatus(nextStatus);
        setErrorMessage('');
      }
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, connectionFailed));
    }
  };

  return {
    handleRefreshPurchaseStatus: () => runPurchaseAction(() => window.go?.app?.App?.RefreshPurchaseStatus?.()),
    handlePurchaseProUnlock: () => runPurchaseAction(() => window.go?.app?.App?.PurchaseProUnlock?.()),
    handleRestorePurchases: () => runPurchaseAction(() => window.go?.app?.App?.RestorePurchases?.()),
  };
}
