package edu

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"time"

	"forgejo.org/modules/log"
)

var validUsernameRe = regexp.MustCompile(`^[\da-zA-Z][-.\w]*$`)

// resolveUniqueUsername finds a unique username by appending numeric suffixes if needed.
// It checks both the current batch (usedInBatch) and existing users in the system.
func (s *service) resolveUniqueUsername(ctx context.Context, base string, usedInBatch map[string]bool) string {
	candidate := base
	suffix := 2
	for {
		if !usedInBatch[candidate] {
			existing, err := s.users.GetUserByName(ctx, candidate)
			if err != nil || existing == nil {
				return candidate
			}
		}
		candidate = fmt.Sprintf("%s%d", base, suffix)
		suffix++
	}
}

func generatePassword() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *service) UploadCSV(ctx context.Context, courseID, creatorID int64, data []byte, mapping CSVColumnMapping) (*ImportDraft, error) {
	rows, err := ParseCSV(data, mapping)
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no valid rows found in CSV")
	}

	now := time.Now().Unix()
	draft := &ImportDraft{
		CourseID:    courseID,
		CreatorID:   creatorID,
		Status:      StatusDraft,
		RawCSV:      string(data),
		CreatedUnix: now,
	}

	if err := s.repo.CreateImportDraft(ctx, draft); err != nil {
		return nil, fmt.Errorf("create import draft: %w", err)
	}

	draftRows := make([]*ImportDraftRow, 0, len(rows))
	for _, r := range rows {
		draftRows = append(draftRows, &ImportDraftRow{
			DraftID:     draft.ID,
			FullName:    r.FullName,
			Email:       r.Email,
			Group:       r.Group,
			Username:    r.Username,
			Role:        "student",
			Status:      StatusPending,
			CreatedUnix: now,
		})
	}

	if err := s.repo.CreateImportDraftRows(ctx, draftRows); err != nil {
		return nil, fmt.Errorf("create import draft rows: %w", err)
	}

	return draft, nil
}

func (s *service) GetImportDraft(ctx context.Context, id int64) (*ImportDraft, []*ImportDraftRow, error) {
	draft, err := s.repo.GetImportDraft(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get import draft: %w", err)
	}
	if draft == nil {
		return nil, nil, nil
	}

	rows, err := s.repo.GetImportDraftRows(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get import draft rows: %w", err)
	}

	return draft, rows, nil
}

func (s *service) UpdateDraftRow(ctx context.Context, rowID int64, username, email, groupName string) error {
	row := &ImportDraftRow{
		ID:       rowID,
		Username: username,
		Email:    email,
		Group:    groupName,
		Status:   StatusPending,
	}
	return s.repo.UpdateImportDraftRow(ctx, row)
}

func (s *service) ExecuteImport(ctx context.Context, draftID int64, doerID int64, defaultRole RoleType) (*ImportResult, error) {
	if s.users == nil {
		return nil, fmt.Errorf("user creator not configured")
	}

	draft, err := s.repo.GetImportDraft(ctx, draftID)
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}
	if draft == nil {
		return nil, fmt.Errorf("draft not found")
	}

	rows, err := s.repo.GetImportDraftRows(ctx, draftID)
	if err != nil {
		return nil, fmt.Errorf("get draft rows: %w", err)
	}

	result := &ImportResult{
		TotalRows: len(rows),
	}

	usedInBatch := map[string]bool{}

	for _, row := range rows {
		if row.Status != StatusPending {
			continue
		}

		if !validUsernameRe.MatchString(row.Username) {
			row.Status = StatusError
			row.ErrorMsg = "invalid username format"
			if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
				log.Error("Failed to update import draft row: %v", errUpd)
			}
			result.Errors++
			continue
		}

		// Check if user already exists by email (if email is provided)
		if row.Email != "" {
			existingByEmail, err := s.users.GetUserByEmail(ctx, row.Email)
			if err == nil && existingByEmail != nil {
				enrollment := &CourseEnrollment{
					CourseID:    draft.CourseID,
					UserID:      existingByEmail.ID,
					Role:        defaultRole,
					GroupName:   row.Group,
					CreatedUnix: time.Now().Unix(),
				}
				if err := s.repo.EnrollUser(ctx, enrollment); err != nil {
					row.Status = StatusError
					row.ErrorMsg = fmt.Sprintf("enroll existing user (by email): %v", err)
					if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
						log.Error("Failed to update import draft row: %v", errUpd)
					}
					result.Errors++
					continue
				}
				if err := s.addToOrgTeam(ctx, draft.CourseID, existingByEmail.ID, defaultRole); err != nil {
					log.Error("Failed to add imported user %d to org team: %v", existingByEmail.ID, err)
				}
				row.Status = "exists"
				if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
					log.Error("Failed to update import draft row: %v", errUpd)
				}
				result.AlreadyExist++
				continue
			}
		}

		// Check if user already exists by username (only if not already claimed in this batch)
		if !usedInBatch[row.Username] {
			existingUser, err := s.users.GetUserByName(ctx, row.Username)
			if err == nil && existingUser != nil {
				// User exists — just enroll
				enrollment := &CourseEnrollment{
					CourseID:    draft.CourseID,
					UserID:      existingUser.ID,
					Role:        defaultRole,
					GroupName:   row.Group,
					CreatedUnix: time.Now().Unix(),
				}
				if err := s.repo.EnrollUser(ctx, enrollment); err != nil {
					row.Status = StatusError
					row.ErrorMsg = fmt.Sprintf("enroll existing user: %v", err)
					if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
						log.Error("Failed to update import draft row: %v", errUpd)
					}
					result.Errors++
					continue
				}
				if err := s.addToOrgTeam(ctx, draft.CourseID, existingUser.ID, defaultRole); err != nil {
					log.Error("Failed to add imported user %d to org team: %v", existingUser.ID, err)
				}
				row.Status = "exists"
				if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
					log.Error("Failed to update import draft row: %v", errUpd)
				}
				usedInBatch[row.Username] = true
				result.AlreadyExist++
				continue
			}
		}

		// Resolve unique username for new user creation (handles batch and system duplicates)
		row.Username = s.resolveUniqueUsername(ctx, row.Username, usedInBatch)
		usedInBatch[row.Username] = true

		// Generate email if empty
		email := row.Email
		if email == "" {
			email = row.Username + "@edu.local"
		}

		// Generate password
		password, err := generatePassword()
		if err != nil {
			row.Status = StatusError
			row.ErrorMsg = fmt.Sprintf("generate password: %v", err)
			if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
				log.Error("Failed to update import draft row: %v", errUpd)
			}
			result.Errors++
			continue
		}

		// Create user
		if err := s.users.CreateUser(ctx, row.Username, email, password, row.FullName); err != nil {
			row.Status = StatusError
			row.ErrorMsg = fmt.Sprintf("create user: %v", err)
			if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
				log.Error("Failed to update import draft row: %v", errUpd)
			}
			result.Errors++
			continue
		}

		// Get the created user for enrollment
		newUser, err := s.users.GetUserByName(ctx, row.Username)
		if err != nil {
			row.Status = StatusError
			row.ErrorMsg = fmt.Sprintf("get created user: %v", err)
			if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
				log.Error("Failed to update import draft row: %v", errUpd)
			}
			result.Errors++
			continue
		}

		// Enroll in course
		enrollment := &CourseEnrollment{
			CourseID:    draft.CourseID,
			UserID:      newUser.ID,
			Role:        defaultRole,
			GroupName:   row.Group,
			CreatedUnix: time.Now().Unix(),
		}
		if err := s.repo.EnrollUser(ctx, enrollment); err != nil {
			row.Status = StatusError
			row.ErrorMsg = fmt.Sprintf("enroll new user: %v", err)
			if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
				log.Error("Failed to update import draft row: %v", errUpd)
			}
			result.Errors++
			continue
		}
		if err := s.addToOrgTeam(ctx, draft.CourseID, newUser.ID, defaultRole); err != nil {
			log.Error("Failed to add imported user %d to org team: %v", newUser.ID, err)
		}

		row.Status = "created"
		if errUpd := s.repo.UpdateImportDraftRow(ctx, row); errUpd != nil {
			log.Error("Failed to update import draft row: %v", errUpd)
		}
		result.Created++
		result.Credentials = append(result.Credentials, UserCredential{
			Username: row.Username,
			Password: password,
			FullName: row.FullName,
			Email:    email,
		})
	}

	draft.Status = StatusDone
	if err := s.repo.UpdateImportDraft(ctx, draft); err != nil {
		log.Error("Failed to update import draft: %v", err)
	}

	return result, nil
}

func (s *service) DeleteImportDraft(ctx context.Context, id int64) error {
	return s.repo.DeleteImportDraft(ctx, id)
}
