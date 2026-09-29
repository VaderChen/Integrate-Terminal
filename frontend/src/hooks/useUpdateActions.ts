import { useRef, useState } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { getMessages, type Locale } from '../i18n';
import type { UpdateActionResult, UpdateCheckResult, UpdateProgress } from '../types';

export function useUpdateActions(locale: Locale) {
  const t = getMessages(locale);
  const running = useRef(false);
  const checkingRef = useRef(false);
  const [checking, setChecking] = useState(false);
  const [feedback, setFeedback] = useState('');
  const [result, setResult] = useState<UpdateCheckResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [actionResult, setActionResult] = useState<UpdateActionResult | null>(null);
  const [error, setError] = useState('');
  const [progress, setProgress] = useState<UpdateProgress | null>(null);
  const check = async () => {
    if (checkingRef.current || running.current) return;
    checkingRef.current = true;
    setChecking(true); setFeedback('');
    try {
      const api = window.go?.app?.App;
      if (!api?.CheckForUpdates) throw new Error(t.settingsUpdateFailed);
      const next = await api.CheckForUpdates();
      setError(''); setActionResult(null); setProgress(null);
      if (next.updateAvailable) setResult(next);
      else setFeedback(t.settingsUpdateUpToDate(next.currentVersion));
    } catch (e) { setFeedback(String(e)); }
    finally { checkingRef.current = false; setChecking(false); }
  };
  const start = async () => {
    if (!result || running.current) return;
    running.current = true;
    setBusy(true); setError(''); setActionResult(null);
    const requestID = crypto.randomUUID();
    setProgress({ requestID, stage: 'preparing', downloadedBytes: 0, totalBytes: 0 });
    let dispose: (() => void) | undefined;
    try {
      const api = window.go?.app?.App;
      if (!api?.StartUpdate) throw new Error(t.settingsUpdateFailed);
      // 先訂閱再呼叫，並以本次 ID 排除重試之前延遲抵達的事件。
      dispose = EventsOn('update:progress', (next: UpdateProgress) => {
        if (next.requestID === requestID) setProgress(next);
      });
      setActionResult(await api.StartUpdate(result.latestTag, requestID));
    } catch (e) { setError(String(e)); }
    finally { dispose?.(); running.current = false; setBusy(false); setProgress(null); }
  };
  const close = () => { if (!running.current) { setResult(null); setProgress(null); } };
  return { checking, feedback, result, busy, actionResult, error, progress, check, start, close };
}
