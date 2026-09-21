import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faCircleInfo, faXmark } from '@fortawesome/free-solid-svg-icons';

type Props = {
  label: string;
  children: ReactNode;
  triggerText?: string;
  closeLabel?: string;
};

// 共用說明泡泡：立即顯示於獨立圖層，不改變原版面，也不受捲動容器裁切。
export function InfoBubble({ label, children, triggerText, closeLabel }: Props) {
  const id = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const bubble = useRef<HTMLDivElement>(null);
  const hideTimer = useRef<ReturnType<typeof setTimeout>>();
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);
  const interactive = Boolean(triggerText);
  const cancelHide = () => clearTimeout(hideTimer.current);
  const show = () => { cancelHide(); setOpen(true); };
  const dismiss = () => {
    cancelHide();
    if (bubble.current?.contains(document.activeElement)) trigger.current?.focus();
    setOpen(false);
    setPosition(null);
  };
  const contains = (target: Node | null) => Boolean(target && (trigger.current?.contains(target) || bubble.current?.contains(target)));
  const leave = () => {
    cancelHide();
    // 留出穿過觸發按鈕與泡泡間隙的時間，顯示本身沒有延遲。
    hideTimer.current = setTimeout(() => {
      if (!contains(document.activeElement)) dismiss();
    }, 120);
  };

  useEffect(() => () => cancelHide(), []);
  useLayoutEffect(() => {
    if (!open) return;
    const update = () => {
      if (!trigger.current || !bubble.current) return;
      const anchor = trigger.current.getBoundingClientRect();
      const panel = bubble.current.getBoundingClientRect();
      const { clientWidth: width, clientHeight: height } = document.documentElement;
      const left = Math.max(12, Math.min(anchor.left, width - panel.width - 12));
      const top = anchor.bottom + 8 + panel.height <= height - 12
        ? anchor.bottom + 8
        : Math.max(12, anchor.top - panel.height - 8);
      setPosition(previous => previous?.left === left && previous.top === top ? previous : { left, top });
    };
    const keydown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      event.stopPropagation();
      dismiss();
    };
    const outside = (event: PointerEvent) => { if (!contains(event.target as Node)) dismiss(); };
    update();
    window.addEventListener('resize', update);
    window.addEventListener('scroll', update, true);
    document.addEventListener('keydown', keydown, true);
    document.addEventListener('pointerdown', outside, true);
    return () => {
      window.removeEventListener('resize', update);
      window.removeEventListener('scroll', update, true);
      document.removeEventListener('keydown', keydown, true);
      document.removeEventListener('pointerdown', outside, true);
    };
  }, [open, children]);

  const portalRoot = trigger.current?.closest('.app-shell') ?? document.body;
  return (
    <>
      <button
        ref={trigger} type="button" className={`info-bubble-trigger${interactive ? ' info-bubble-link' : ''}`}
        aria-label={label} aria-describedby={open && !interactive ? id : undefined}
        aria-haspopup={interactive ? 'dialog' : undefined} aria-expanded={interactive ? open : undefined}
        aria-controls={open && interactive ? id : undefined}
        onMouseEnter={show} onMouseLeave={leave} onFocus={show} onClick={show}
        onKeyDown={event => {
          if (interactive && open && (event.key === 'ArrowDown' || (event.key === 'Tab' && !event.shiftKey))) {
            const firstAction = bubble.current?.querySelector<HTMLButtonElement>('button');
            if (firstAction) { event.preventDefault(); firstAction.focus(); }
          }
        }}
        onBlur={event => { if (!contains(event.relatedTarget)) dismiss(); }}
      >
        {triggerText}<FontAwesomeIcon icon={faCircleInfo} />
      </button>
      {open ? createPortal(
        <div
          ref={bubble} id={id} role={interactive ? 'dialog' : 'tooltip'} aria-label={interactive ? label : undefined}
          className={`info-bubble${interactive ? ' info-bubble-document' : ''}`}
          style={{ left: position?.left ?? 0, top: position?.top ?? 0, visibility: position ? 'visible' : 'hidden' }}
          onMouseEnter={cancelHide} onMouseLeave={leave}
          onBlur={event => { if (!contains(event.relatedTarget)) dismiss(); }}
          onClick={event => event.stopPropagation()}
        >
          {interactive ? <div className="info-bubble-heading"><strong>{label}</strong><button type="button" className="info-bubble-close" onClick={dismiss} aria-label={closeLabel}><FontAwesomeIcon icon={faXmark} /></button></div> : null}
          {children}
        </div>, portalRoot,
      ) : null}
    </>
  );
}
