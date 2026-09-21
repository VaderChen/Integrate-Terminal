import { useEffect, useRef, useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faCopy, faDownload, faEye, faEyeSlash } from '@fortawesome/free-solid-svg-icons';
import type { Config, RestServerStatus } from '../types';
import { type Locale, useI18n } from '../i18n';
import { mcpClientConfig, parseMCPPort, redactMCPToken } from './mcpSettings';
import { InfoBubble } from './InfoBubble';

type Props = {
  config: Config;
  locale: Locale;
  onRESTServerEnabledChange: (enabled: boolean, port?: number) => Promise<void> | void;
  onRESTServerPortChange: (port: number) => Promise<void> | void;
  onShowTrayIconChange: (enabled: boolean) => Promise<void> | void;
};

export function MCPSettingsPanel({ config, locale, onRESTServerEnabledChange, onRESTServerPortChange, onShowTrayIconChange }: Props) {
  const t = useI18n(locale);
  const [contract, setContract] = useState<'local' | 'network'>('local');
  const [markdown, setMarkdown] = useState('');
  const [executable, setExecutable] = useState('');
  const [status, setStatus] = useState<RestServerStatus | null>(null);
  const [token, setToken] = useState('');
  const [tokenVisible, setTokenVisible] = useState(false);
  const [portDraft, setPortDraft] = useState(String(config.restServerPort));
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [feedback, setFeedback] = useState('');
  const generation = useRef(0);
  const mounted = useRef(false);
  const mutationPending = useRef(false);

  const errorText = (cause: unknown) => cause instanceof Error ? cause.message : typeof cause === 'string' ? cause : t.connectionFailed;
  const refresh = async () => {
    const request = ++generation.current;
    setLoading(true);
    try {
      const api = window.go?.app?.App;
      if (!api?.GetRestAPIDocsMarkdown || !api.GetRESTServerStatus || !api.GetRESTServerToken || !api.GetMCPStdioExecutable) throw new Error(t.connectionFailed);
      const [nextMarkdown, nextStatus, nextToken, nextExecutable] = await Promise.all([
        api.GetRestAPIDocsMarkdown(), api.GetRESTServerStatus(), api.GetRESTServerToken(), api.GetMCPStdioExecutable(),
      ]);
      if (!mounted.current || request !== generation.current) return;
      // Older backends may still interpolate the token. Keep every document surface safe.
      setMarkdown(redactMCPToken(nextMarkdown, nextToken));
      setStatus(nextStatus);
      setToken(nextToken);
      setExecutable(nextExecutable);
    } catch (cause) {
      if (mounted.current && request === generation.current) {
        setMarkdown('');
        setToken('');
        setExecutable('');
        setStatus(null);
        setError(errorText(cause));
      }
    } finally {
      if (mounted.current && request === generation.current) setLoading(false);
    }
  };

  useEffect(() => {
    mounted.current = true;
    setError('');
    void refresh();
    return () => { mounted.current = false; generation.current++; };
  }, [config.restServerEnabled, config.restServerPort, locale]);

  useEffect(() => { setPortDraft(String(config.restServerPort)); }, [config.restServerPort]);
  useEffect(() => { setTokenVisible(false); setFeedback(''); }, [contract]);

  const endpoint = status?.mcpURL || `${status?.baseURL || `http://127.0.0.1:${config.restServerPort}`}/mcp`;
  const clientConfig = mcpClientConfig(contract, executable, endpoint);
  const copy = async (value: string, isToken = false) => {
    try {
      await navigator.clipboard.writeText(value);
      if (mounted.current) setFeedback(isToken ? t.settingsRestServerTokenCopySuccess : t.settingsMcpCopySuccess);
    } catch {
      if (mounted.current) setError(isToken ? t.settingsRestServerTokenCopyFailed : t.settingsSkillCopyFailed);
    }
  };

  const save = async (change: () => Promise<void> | void) => {
    if (mutationPending.current) return;
    mutationPending.current = true;
    setSaving(true);
    setError('');
    try {
      await change();
      // Parent preferences update optimistically; query again after the native save completes.
      if (mounted.current) await refresh();
    } catch (cause) {
      if (mounted.current) { setError(errorText(cause)); await refresh(); }
    } finally {
      mutationPending.current = false;
      if (mounted.current) setSaving(false);
    }
  };

  const commitPort = () => {
    const port = parseMCPPort(portDraft);
    if (port === null) { setError(t.settingsMcpPortInvalid); return; }
    if (port !== config.restServerPort) void save(() => onRESTServerPortChange(port));
  };

  const toggleHTTP = () => {
    const port = parseMCPPort(portDraft);
    if (!config.restServerEnabled && port === null) { setError(t.settingsMcpPortInvalid); return; }
    void save(() => onRESTServerEnabledChange(!config.restServerEnabled, port ?? config.restServerPort));
  };

  const exportMarkdown = async () => {
    try {
      const exportDocs = window.go?.app?.App?.ExportRestAPIDocsMarkdown;
      if (!exportDocs) throw new Error(t.connectionFailed);
      await exportDocs();
    } catch (cause) { if (mounted.current) setError(errorText(cause)); }
  };

  return (
    <div className="settings-section-card settings-section-stack settings-skill-card">
      <div className="settings-mcp-label"><strong>{t.settingsSkillTitle}</strong><InfoBubble label={t.settingsNavSkill}><p>{t.settingsSkillHint}</p><p>{t.settingsMcpContractSeparationHint}</p></InfoBubble></div>
      <div className="settings-mcp-tabs" role="tablist" aria-label={t.settingsNavSkill}>
        <button type="button" role="tab" aria-selected={contract === 'local'} className={`settings-mcp-tab ${contract === 'local' ? 'active' : ''}`} onClick={() => setContract('local')}>{t.settingsMcpLocalTab}</button>
        <button type="button" role="tab" aria-selected={contract === 'network'} className={`settings-mcp-tab ${contract === 'network' ? 'active' : ''}`} onClick={() => setContract('network')}>{t.settingsMcpNetworkTab}</button>
      </div>
      {contract === 'local' ? (
        <div className="settings-section-card settings-section-stack">
          <div className="settings-mcp-label"><strong>{t.settingsMcpLocalTitle}</strong><InfoBubble label={t.settingsMcpLocalTitle}>{t.settingsMcpLocalHint}</InfoBubble></div>
          <div className="settings-mcp-endpoint-row"><span>{t.settingsMcpLocalVirtualRoot}</span><code>integterm-vfs://workspace/mcp</code></div>
        </div>
      ) : (
        <>
          <div className="settings-section-card settings-rest-server-row">
            <div className="settings-section-copy"><strong>{t.settingsRestServer}</strong><span>{loading ? t.loading : status?.running ? status.attached ? t.settingsRestServerStatusAttached(endpoint) : t.settingsRestServerStatusRunning(endpoint) : t.settingsRestServerStatusStopped(config.restServerPort)}</span></div>
            <div className="settings-rest-server-controls">
              <InfoBubble label={t.settingsRestServerPort}>{t.settingsRestServerPortHint}</InfoBubble>
              <div className="settings-rest-port-field"><input type="number" min={1} max={65535} value={portDraft} disabled={config.restServerEnabled || saving} aria-label={t.settingsRestServerPort} onChange={event => setPortDraft(event.target.value)} onBlur={event => { if (!(event.relatedTarget as HTMLElement | null)?.hasAttribute('data-mcp-toggle')) commitPort(); }} onKeyDown={event => { if (event.key === 'Enter') { event.preventDefault(); commitPort(); } }} /></div>
              <button type="button" data-mcp-toggle role="switch" aria-checked={config.restServerEnabled} aria-label={t.settingsRestServer} className={`ios-switch ${config.restServerEnabled ? 'active' : ''}`} disabled={saving || loading} onClick={toggleHTTP} title={config.restServerEnabled ? t.settingsOn : t.settingsOff}><span className="ios-switch-track" /><span className="ios-switch-thumb" /></button>
            </div>
          </div>
          <div className="settings-section-card settings-rest-token-card">
            <div className="settings-mcp-label"><strong>{t.settingsRestServerToken}</strong><InfoBubble label={t.settingsRestServerToken}>{t.settingsRestServerTokenHint}</InfoBubble></div>
            <div className="settings-rest-token-controls">
              <div className="settings-rest-token-field">
                <input type={tokenVisible ? 'text' : 'password'} value={token} readOnly autoComplete="off" spellCheck={false} aria-label={t.settingsRestServerToken} placeholder={loading ? t.loading : t.settingsRestServerTokenUnavailable} />
                <button type="button" className="settings-rest-token-visibility" onClick={() => setTokenVisible(value => !value)} disabled={!token || loading} aria-label={tokenVisible ? t.settingsRestServerTokenHide : t.settingsRestServerTokenShow} title={tokenVisible ? t.settingsRestServerTokenHide : t.settingsRestServerTokenShow}><FontAwesomeIcon icon={tokenVisible ? faEyeSlash : faEye} /></button>
              </div>
              <button type="button" className="settings-rest-token-copy" onClick={() => void copy(token, true)} disabled={!token || loading} aria-label={t.settingsRestServerTokenCopy} title={t.settingsRestServerTokenCopy}><FontAwesomeIcon icon={faCopy} /></button>
            </div>
          </div>
          <div className="settings-section-card">
            <div className="settings-mcp-label"><strong>{t.settingsShowTrayIcon}</strong><InfoBubble label={t.settingsShowTrayIcon}><p>{t.settingsShowTrayIconHint}</p>{config.restServerEnabled ? <p>{t.settingsShowTrayIconRequired}</p> : null}</InfoBubble></div>
            <button type="button" role="switch" aria-checked={config.showTrayIcon} aria-label={t.settingsShowTrayIcon} className={`ios-switch ${config.showTrayIcon ? 'active' : ''}`} disabled={saving || config.restServerEnabled} onClick={() => void save(() => onShowTrayIconChange(!config.showTrayIcon))} title={config.showTrayIcon ? t.settingsOn : t.settingsOff}><span className="ios-switch-track" /><span className="ios-switch-thumb" /></button>
          </div>
        </>
      )}
      <div className="settings-mcp-config-section">
        <div className="settings-mcp-config-heading">
          <div className="settings-mcp-label"><strong>{t.settingsMcpClientConfig}</strong><InfoBubble label={t.settingsMcpClientConfig}>{contract === 'network' ? t.settingsMcpHTTPConfigHint : t.settingsMcpLocalConfigHint}</InfoBubble></div>
          <button type="button" className="settings-skill-action-button" disabled={loading || !executable} onClick={() => void copy(clientConfig)} aria-label={t.settingsMcpCopyConfig} title={t.settingsMcpCopyConfig}><FontAwesomeIcon icon={faCopy} /></button>
        </div>
        <pre className="settings-mcp-config">{loading ? t.loading : executable ? clientConfig : t.settingsSkillEmpty}</pre>
      </div>
      <div className="settings-mcp-docs-action">
        <InfoBubble label={t.settingsSkillTitle} triggerText={t.settingsSkillTitle} closeLabel={t.close}>
          <pre className="settings-skill-viewer">{loading ? t.loading : markdown || t.settingsSkillEmpty}</pre>
          <div className="settings-skill-actions">
            <button type="button" className="settings-skill-action-button" disabled={loading || !markdown} onClick={() => void copy(markdown)} aria-label={t.settingsSkillCopy} title={t.settingsSkillCopy}><FontAwesomeIcon icon={faCopy} /></button>
            <button type="button" className="settings-skill-action-button accent" disabled={loading || !markdown} onClick={() => void exportMarkdown()} aria-label={t.settingsSkillExport} title={t.settingsSkillExport}><FontAwesomeIcon icon={faDownload} /></button>
          </div>
        </InfoBubble>
      </div>
      {error ? <span className="settings-skill-feedback error" role="alert">{error}</span> : null}
      {feedback ? <span className="settings-skill-feedback success" role="status">{feedback}</span> : null}
    </div>
  );
}
