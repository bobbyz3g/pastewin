package main

import (
	"errors"
	"fmt"

	"pastewin/internal/clipimage"
	"pastewin/internal/native"
)

type captureJob struct {
	snapshot native.Snapshot
	epoch    uint64
}

func (s *ClipboardService) capture() {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if s.paused || s.ctx.Err() != nil {
		return
	}
	if seq := native.Sequence(); seq == s.ignoreSequence || seq == s.lastCaptured {
		return
	}
	snapshot, err := s.monitor.ReadSnapshot()
	if err != nil {
		s.report(err)
		return
	}
	if snapshot.Sequence == s.ignoreSequence || snapshot.Sequence == s.lastCaptured {
		return
	}
	s.lastCaptured = snapshot.Sequence
	if snapshot.Text == "" && len(snapshot.Images) == 0 {
		if snapshot.Warning != nil {
			s.report(snapshot.Warning)
		}
		return
	}
	job := captureJob{snapshot, s.captureEpoch}
	select {
	case s.captures <- job:
	default:
		// One decoding job and at most one queued snapshot bound image memory.
		// When overwhelmed, retain the newest snapshot instead of growing a backlog.
		select {
		case <-s.captures:
			s.report(errors.New("复制过快，已跳过一条尚未处理的记录"))
		default:
		}
		s.captures <- job
	}
}

func (s *ClipboardService) processCaptures() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case job := <-s.captures:
			s.opMu.Lock()
			valid := !s.paused && job.epoch == s.captureEpoch && s.ctx.Err() == nil
			s.opMu.Unlock()
			if !valid {
				continue
			}
			img, err := clipimage.Prepare(job.snapshot.Images)
			s.commitCapture(job, img, err)
		}
	}
}

func (s *ClipboardService) commitCapture(job captureJob, img *clipimage.Image, decodeErr error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	// Pause/resume invalidates work that was decoding outside the operation lock.
	if s.ctx.Err() != nil || s.paused || s.captureEpoch != job.epoch {
		return
	}
	if decodeErr != nil {
		s.report(fmt.Errorf("图片未保存：%w", decodeErr))
	}
	if job.snapshot.Warning != nil {
		s.report(job.snapshot.Warning)
	}
	if err := s.store.AddContent(job.snapshot.Text, img); err != nil {
		s.report(err)
		return
	}
	s.changed()
}
