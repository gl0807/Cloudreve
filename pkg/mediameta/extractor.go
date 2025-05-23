package mediameta

import (
	"context"
	"encoding/gob"
	"errors"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/driver"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/manager/entitysource"
	"github.com/cloudreve/Cloudreve/v4/pkg/logging"
	"github.com/cloudreve/Cloudreve/v4/pkg/setting"
	"github.com/samber/lo"
	"io"
)

type (
	Extractor interface {
		// Exts returns the supported file extensions.
		Exts() []string
		// Extract extracts the media meta from the given source.
		Extract(ctx context.Context, ext string, source entitysource.EntitySource) ([]driver.MediaMeta, error)
	}
)

var (
	ErrFileTooLarge = errors.New("file too large")
)

func init() {
	gob.Register([]driver.MediaMeta{})
}

func NewExtractorManager(ctx context.Context, settings setting.Provider, l logging.Logger) Extractor {
	e := &extractorManager{
		settings: settings,
		extMap:   make(map[string][]Extractor),
	}

	extractors := []Extractor{}

	if e.settings.MediaMetaExifEnabled(ctx) {
		exifE := newExifExtractor(settings, l)
		extractors = append(extractors, exifE)
	}

	if e.settings.MediaMetaMusicEnabled(ctx) {
		musicE := newMusicExtractor(settings, l)
		extractors = append(extractors, musicE)
	}

	if e.settings.MediaMetaFFProbeEnabled(ctx) {
		ffprobeE := newFFProbeExtractor(settings, l)
		extractors = append(extractors, ffprobeE)
	}

	for _, extractor := range extractors {
		for _, ext := range extractor.Exts() {
			if e.extMap[ext] == nil {
				e.extMap[ext] = []Extractor{}
			}
			e.extMap[ext] = append(e.extMap[ext], extractor)
		}
	}

	return e
}

type extractorManager struct {
	settings setting.Provider
	extMap   map[string][]Extractor
}

func (e *extractorManager) Exts() []string {
	return lo.Keys(e.extMap)
}

func (e *extractorManager) Extract(ctx context.Context, ext string, source entitysource.EntitySource) ([]driver.MediaMeta, error) {
	if extractor, ok := e.extMap[ext]; ok {
		res := []driver.MediaMeta{}
		for _, e := range extractor {
			_, _ = source.Seek(0, io.SeekStart)
			data, err := e.Extract(ctx, ext, source)
			if err != nil {
				return nil, err
			}

			res = append(res, data...)
		}

		return res, nil
	} else {
		return nil, nil
	}
}

// checkFileSize checks if the file size exceeds the limit.
func checkFileSize(localLimit, remoteLimit int64, source entitysource.EntitySource) error {
	if source.IsLocal() && localLimit > 0 && source.Entity().Size() > localLimit {
		return ErrFileTooLarge
	}

	if !source.IsLocal() && remoteLimit > 0 && source.Entity().Size() > remoteLimit {
		return ErrFileTooLarge
	}

	return nil
}
