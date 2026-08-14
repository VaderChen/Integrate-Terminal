package transport

import (
	"errors"
	"io"
	"time"
)

type progressReader struct {
	reader          io.Reader
	total           int64
	progress        func(transferred int64, total int64, speedBps int64) bool
	transferred     int64
	lastTransferred int64
	lastReportTime  time.Time
}

var ErrTransferCancelled = errors.New("transfer cancelled")

type progressWriter struct {
	writer          io.Writer
	total           int64
	progress        func(transferred int64, total int64, speedBps int64) bool
	transferred     int64
	lastTransferred int64
	lastReportTime  time.Time
}

func newProgressReader(reader io.Reader, total int64, progress func(transferred int64, total int64, speedBps int64) bool) io.Reader {
	if progress == nil {
		return reader
	}
	now := time.Now()
	if !progress(0, total, 0) {
		return &cancelledReader{}
	}
	return &progressReader{
		reader:         reader,
		total:          total,
		progress:       progress,
		lastReportTime: now,
	}
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.transferred += int64(n)
		now := time.Now()
		elapsed := now.Sub(r.lastReportTime)
		if elapsed >= 250*time.Millisecond || err == io.EOF {
			bytesSinceLast := r.transferred - r.lastTransferred
			speedBps := int64(0)
			if elapsed > 0 {
				speedBps = int64(float64(bytesSinceLast) / elapsed.Seconds())
			}
			if !r.progress(r.transferred, r.total, speedBps) {
				return n, ErrTransferCancelled
			}
			r.lastReportTime = now
			r.lastTransferred = r.transferred
		}
	}
	if err == io.EOF && r.transferred == r.total {
		if !r.progress(r.transferred, r.total, 0) {
			return n, ErrTransferCancelled
		}
	}
	return n, err
}

func newProgressWriter(writer io.Writer, total int64, progress func(transferred int64, total int64, speedBps int64) bool) io.Writer {
	if progress == nil {
		return writer
	}
	now := time.Now()
	if !progress(0, total, 0) {
		return &cancelledWriter{}
	}
	return &progressWriter{
		writer:         writer,
		total:          total,
		progress:       progress,
		lastReportTime: now,
	}
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.transferred += int64(n)
		now := time.Now()
		elapsed := now.Sub(w.lastReportTime)
		reachedEnd := w.total > 0 && w.transferred >= w.total
		if elapsed >= 120*time.Millisecond || reachedEnd {
			bytesSinceLast := w.transferred - w.lastTransferred
			speedBps := int64(0)
			if elapsed > 0 {
				speedBps = int64(float64(bytesSinceLast) / elapsed.Seconds())
			}
			if !w.progress(w.transferred, w.total, speedBps) {
				return n, ErrTransferCancelled
			}
			w.lastReportTime = now
			w.lastTransferred = w.transferred
		}
	}
	if err == nil && w.total > 0 && w.transferred >= w.total {
		if !w.progress(w.transferred, w.total, 0) {
			return n, ErrTransferCancelled
		}
	}
	return n, err
}

type cancelledReader struct{}

func (r *cancelledReader) Read(_ []byte) (int, error) {
	return 0, ErrTransferCancelled
}

type cancelledWriter struct{}

func (w *cancelledWriter) Write(_ []byte) (int, error) {
	return 0, ErrTransferCancelled
}
