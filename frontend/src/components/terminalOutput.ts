export type TerminalOutputSnapshot = { output: string; sequence: number };

/** Subscribe before taking the snapshot; sequence numbers join both streams without gaps or duplicates. */
export function attachTerminalOutput(options: {
  subscribe: (listener: (chunk: string, sequence: number) => void) => () => void;
  readSnapshot: () => Promise<TerminalOutputSnapshot>;
  write: (chunk: string, replay: boolean) => void;
}) {
  let disposed = false;
  let initialized = false;
  let sequence = 0;
  const pending = new Map<number, string>();
  const flush = () => {
    let chunk: string | undefined;
    while ((chunk = pending.get(sequence + 1)) !== undefined) {
      sequence += 1;
      pending.delete(sequence);
      options.write(chunk, false);
    }
  };
  const unsubscribe = options.subscribe((chunk, nextSequence) => {
    if (disposed || !Number.isSafeInteger(nextSequence) || nextSequence <= sequence) return;
    if (initialized && pending.size === 0 && nextSequence === sequence + 1) {
      // 正常連續輸出直接寫入；只有快照尚未完成或序號缺口需要暫存。
      sequence = nextSequence;
      options.write(chunk, false);
    } else {
      pending.set(nextSequence, chunk);
      if (initialized) flush();
    }
  });
  const ready = (async () => {
    const snapshot = await options.readSnapshot();
    if (disposed) return;
    sequence = snapshot.sequence;
    if (snapshot.output) options.write(snapshot.output, true);
    for (const key of pending.keys()) if (key <= sequence) pending.delete(key);
    initialized = true;
    flush();
  })();
  return {
    ready,
    dispose: () => {
      disposed = true;
      unsubscribe();
      pending.clear();
    },
  };
}
