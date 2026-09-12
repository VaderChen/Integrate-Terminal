import { useCallback, useEffect, useRef, useState } from 'react';
import { getMessages, type Locale } from '../i18n';
import type { UpdateActionResult, UpdateCheckResult } from '../types';

const AUTO_UPDATE_NEXT_CHECK_KEY = 'integterm.update.nextCheckAt';
const MAX_TIMER_DELAY_MS = 2_147_000_000;
let volatileNextCheckAt: number | null = null;

type CheckMode = 'manual' | 'automatic';

type Params = {
  enabled: boolean;
  locale: Locale;
};

export function useUpdateActions({ enabled, locale }: Params) {
  const t = getMessages(locale);
  const checkingRef = useRef(false);
  const [checking, setChecking] = useState(false);
  const [feedback, setFeedback] = useState('');
  const [feedbackError, setFeedbackError] = useState(false);
  const [checkResult, setCheckResult] = useState<UpdateCheckResult | null>(null);
  const [actionBusy, setActionBusy] = useState(false);
  const [actionResult, setActionResult] = useState<UpdateActionResult | null>(null);
  const [actionError, setActionError] = useState('');

  const checkForUpdates = useCallback(async (mode: CheckMode = 'manual') => {
    if (checkingRef.current) {
      return null;
    }

    checkingRef.current = true;
    setChecking(true);
    if (mode === 'manual') {
      setFeedback('');
      setFeedbackError(false);
    }
    setActionResult(null);
    setActionError('');

    try {
      const result = await window.go?.app?.App?.CheckForUpdates?.();
      if (!result) {
        throw new Error('update check returned no result');
      }
      if (result.updateAvailable) {
        setCheckResult(result);
      } else if (mode === 'manual') {
        setCheckResult(null);
        setFeedback(t.settingsUpdateUpToDate(result.currentVersion));
      }
      return result;
    } catch {
      if (mode === 'manual') {
        setCheckResult(null);
        setFeedback(t.settingsUpdateFailed);
        setFeedbackError(true);
      }
      return null;
    } finally {
      checkingRef.current = false;
      setChecking(false);
    }
  }, [t]);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    let cancelled = false;
    let timer: number | undefined;

    const runScheduledCheck = async (): Promise<void> => {
      if (cancelled) {
        return;
      }
      await checkForUpdates('automatic');
      if (cancelled) {
        return;
      }
      writeNextCheckAt(randomTimeTomorrow(Date.now()));
      scheduleNextCheck();
    };

    const scheduleNextCheck = (): void => {
      if (cancelled) {
        return;
      }

      const now = Date.now();
      const latestValidSchedule = endOfTomorrow(now);
      let nextCheckAt = readNextCheckAt();
      if (nextCheckAt === null || nextCheckAt > latestValidSchedule) {
        nextCheckAt = randomTimeBeforeMidnight(now);
        writeNextCheckAt(nextCheckAt);
      }

      const delay = nextCheckAt - now;
      if (delay <= 0) {
        void runScheduledCheck();
        return;
      }

      timer = window.setTimeout(
        delay > MAX_TIMER_DELAY_MS ? scheduleNextCheck : () => void runScheduledCheck(),
        Math.min(delay, MAX_TIMER_DELAY_MS),
      );
    };

    scheduleNextCheck();
    return () => {
      cancelled = true;
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [checkForUpdates, enabled]);

  const closeDialog = useCallback(() => {
    if (actionBusy) {
      return;
    }
    setCheckResult(null);
    setActionResult(null);
    setActionError('');
  }, [actionBusy]);

  const startUpdate = useCallback(async () => {
    if (!checkResult || actionBusy) {
      return;
    }
    try {
      setActionBusy(true);
      setActionResult(null);
      setActionError('');
      const result = await window.go?.app?.App?.StartUpdate?.(checkResult.latestTag);
      if (!result) {
        throw new Error('update action returned no result');
      }
      setActionResult(result);
    } catch {
      setActionError(t.settingsUpdateFailed);
    } finally {
      setActionBusy(false);
    }
  }, [actionBusy, checkResult, t.settingsUpdateFailed]);

  return {
    checking,
    feedback,
    feedbackError,
    checkResult,
    actionBusy,
    actionResult,
    actionError,
    checkForUpdates,
    closeDialog,
    startUpdate,
  };
}

function readNextCheckAt(): number | null {
  try {
    const value = Number(window.localStorage.getItem(AUTO_UPDATE_NEXT_CHECK_KEY));
    return Number.isFinite(value) && value > 0 ? value : volatileNextCheckAt;
  } catch {
    return volatileNextCheckAt;
  }
}

function writeNextCheckAt(value: number) {
  volatileNextCheckAt = value;
  try {
    window.localStorage.setItem(AUTO_UPDATE_NEXT_CHECK_KEY, String(value));
  } catch {}
}

function randomTimeBeforeMidnight(now: number): number {
  return randomTimestamp(now, startOfNextDay(now));
}

function randomTimeTomorrow(now: number): number {
  const start = startOfNextDay(now);
  return randomTimestamp(start, startOfNextDay(start));
}

function endOfTomorrow(now: number): number {
  return startOfNextDay(startOfNextDay(now));
}

function startOfNextDay(timestamp: number): number {
  const date = new Date(timestamp);
  date.setHours(24, 0, 0, 0);
  return date.getTime();
}

function randomTimestamp(start: number, end: number): number {
  if (end <= start) {
    return start;
  }
  return start + Math.floor(Math.random() * (end - start));
}
