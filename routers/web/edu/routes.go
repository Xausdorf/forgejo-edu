package edu

import (
	"forgejo.org/modules/setting"
	"forgejo.org/modules/web"
	"forgejo.org/services/context"
)

func RegisterRoutes(m *web.Route, middlewares ...any) {
	m.Group("/edu", func() {
		m.Get("/dashboard", Dashboard)

		m.Group("/student", func() {
			m.Get("/assignments", StudentAssignments)
			m.Get("/assignments/{id}", AssignmentDetail)
		})

		m.Group("/teacher", func() {
			m.Get("/assignments", TeacherAssignments)
			m.Get("/assignments/new", NewAssignment)
			m.Post("/assignments/new", NewAssignmentPost)
			m.Get("/assignments/{id}/edit", EditAssignment)
			m.Post("/assignments/{id}/edit", EditAssignmentPost)
			m.Post("/assignments/{id}/delete", DeleteAssignmentPost)
			m.Post("/assignments/{id}/distribute", DistributePost)
			m.Get("/assignments/{id}/distribute-status", DistributeStatus)
			m.Get("/assignments/{id}/submissions", InstructorSubmissions)
			m.Get("/assignments/{id}/submissions/{subID}", SubmissionDetail)
			m.Post("/assignments/{id}/submissions/{subID}/grade", GradeSubmissionPost)
			m.Post("/assignments/{id}/submissions/{subID}/reset-grade", ResetGradePost)
			m.Get("/dashboard", func(ctx *context.Context) {
				ctx.Redirect(setting.AppSubURL + "/edu/teacher/assignments")
			})

			m.Group("/courses", func() {
				m.Get("", CourseList)
				m.Get("/new", NewCourse)
				m.Post("/new", NewCoursePost)
				m.Get("/{id}", CourseDetail)
				m.Get("/{id}/edit", EditCourse)
				m.Post("/{id}/edit", EditCoursePost)
				m.Post("/{id}/delete", DeleteCoursePost)
				m.Post("/{id}/enroll", EnrollUserPost)
				m.Post("/{id}/unenroll", RemoveEnrollmentPost)
				m.Post("/{id}/init-forks", InitForksPost)
				m.Get("/{id}/init-forks-status", InitForksStatus)
				m.Get("/{id}/import", ImportUpload)
				m.Post("/{id}/import", ImportUploadPost)
				m.Get("/{id}/import/{draftID}/preview", ImportPreview)
				m.Post("/{id}/import/{draftID}/update-row", ImportUpdateRow)
				m.Post("/{id}/import/{draftID}/execute", ImportExecutePost)
				m.Post("/{id}/import/{draftID}/delete", ImportDeletePost)
			})
		}, reqEduTeacher)

		m.Group("/admin", func() {
			m.Get("", AdminPanel)
			m.Post("/roles", web.Bind(EduRoleForm{}), UpdateUserRolePost)
		}, reqEduAdmin)
	}, middlewares...)
}
