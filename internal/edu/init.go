package edu

import (
	"context"
	"embed"
	"fmt"

	"forgejo.org/models/db"
	"forgejo.org/modules/log"
	"forgejo.org/modules/translation/i18n"
)

//go:embed locale/*.json
var localeFS embed.FS

func Init(ctx context.Context) error {
	log.Info("Initializing Educational Extension...")

	e := db.GetEngine(ctx)
	if e == nil {
		return fmt.Errorf("Educational Extension: Database engine not available")
	}

	if err := e.Sync(
		new(Course),
		new(CourseEnrollment),
		new(Assignment),
		new(Submission),
		new(TestResult),
		new(UserRole),
		new(ImportDraft),
		new(ImportDraftRow),
		new(InitForksTask),
		new(DistributeTask),
		new(CourseSyncTask),
		new(CourseSyncPR),
	); err != nil {
		log.Error("Educational Extension: Failed to sync database schema: %v", err)
		return err
	}

	repo := NewRepository()
	adapter := NewForgejoAdapter()
	RegisterNotifier(repo, adapter, adapter, adapter)
	globalService = NewService(repo, adapter, adapter)

	// Load edu-specific locale strings into the global i18n store.
	localeFiles, err := localeFS.ReadDir("locale")
	if err != nil {
		log.Error("Educational Extension: Failed to read embedded locale dir: %v", err)
	} else {
		for _, f := range localeFiles {
			data, err := localeFS.ReadFile("locale/" + f.Name())
			if err != nil {
				log.Error("Educational Extension: Failed to read locale file %s: %v", f.Name(), err)
				continue
			}
			// Extract lang name from filename: locale_en-US.json -> en-US
			name := f.Name()
			lang := name[len("locale_") : len(name)-len(".json")]
			if !i18n.DefaultLocales.HasLang(lang) {
				continue
			}
			if err := i18n.DefaultLocales.AddToLocaleFromJSON(lang, data); err != nil {
				log.Error("Educational Extension: Failed to add locale %s: %v", lang, err)
			}
		}
	}

	log.Info("Educational Extension initialized successfully.")
	return nil
}
