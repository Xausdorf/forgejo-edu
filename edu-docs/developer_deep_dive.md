# Погружение в архитектуру Forgejo и Образовательного расширения

Этот документ предназначен для разработчика, желающего понять внутреннее устройство проекта "с нуля". Он охватывает архитектуру платформы Forgejo и детали реализации образовательного модуля (Edu Extension).

## Содержание

- [Часть 1: Архитектура Forgejo](#часть-1-архитектура-forgejo)
  - [1.1 Структура папок](#11-структура-папок)
  - [1.2 Жизненный цикл запроса](#12-жизненный-цикл-запроса-request-lifecycle)
- [Часть 2: Образовательное расширение](#часть-2-образовательное-расширение-edu-extension)
  - [2.1 Где лежит код?](#21-где-лежит-код)
  - [2.2 Архитектура модуля](#22-архитектура-модуля)
  - [2.3 Модель данных (11 таблиц)](#23-модель-данных-11-таблиц)
  - [2.4 Файлы internal/edu/](#24-файлы-internaledu)
  - [2.5 Ключевые сценарии](#25-ключевые-сценарии)
    - [А. Управление курсами](#а-управление-курсами)
    - [Б. CSV-импорт студентов](#б-csv-импорт-студентов)
    - [В. Создание задания](#в-создание-задания)
    - [Г. Init forks](#г-init-forks-инициализация-студенческих-форков-на-уровне-курса)
    - [Д. Distribute](#д-distribute-раздача-задания)
    - [Е. CI/CD: путь от push студента до PR](#е-cicd-путь-от-push-студента-до-pr)
    - [Ж. Submission review](#ж-submission-review-approve--merge-через-edu-ui)
    - [З. Course sync](#з-course-sync-pr-based-синхронизация-шаблона-в-форки)
    - [И. Auto-grade vs ManualGrade](#и-auto-grade-vs-manualgrade)
    - [К. Маппинг ролей на org](#к-маппинг-ролей-на-org)
    - [Л. Каскадное удаление курса](#л-каскадное-удаление-курса)
    - [М. Ограничение доступа по активности курса](#м-ограничение-доступа-по-активности-курса)
  - [2.6 Полная карта роутов](#26-полная-карта-роутов)
  - [2.7 Интеграция с ядром Forgejo](#27-интеграция-с-ядром-forgejo)
- [Часть 3: Технический стек и паттерны](#часть-3-технический-стек-и-паттерны)
  - [3.1 ORM: Xorm](#31-orm-xorm)
  - [3.2 Фронтенд: Server-side rendering](#32-фронтенд-server-side-rendering)
  - [3.3 Локализация (i18n)](#33-локализация-i18n)
  - [3.4 Локальная разработка (Docker)](#34-локальная-разработка-docker)
  - [3.5 Тестирование](#35-тестирование)
  - [3.6 Ошибки и логирование](#36-ошибки-и-логирование)
  - [3.7 Права доступа и авторизация](#37-права-доступа-и-авторизация)
  - [3.8 Обработка ошибок](#38-обработка-ошибок)
  - [3.9 XSS-защита в шаблонах](#39-xss-защита-в-шаблонах)
- [Часть 4: Как развивать проект](#часть-4-как-развивать-проект)
- [Часть 5: Обновление из upstream Forgejo](#часть-5-обновление-из-upstream-forgejo)

---

## Часть 1: Архитектура Forgejo

![Архитектура Forgejo](forgejo-architecture.png)

Forgejo (форк Gitea) — это монолитное приложение на Go, использующее стандартную трехуровневую архитектуру (MVC).

### 1.1 Структура папок

*   **`cmd/`**: Точки входа (main). Команда `web` запускает сервер.
*   **`routers/`**: **Контроллеры (C)**. Здесь живут HTTP-хендлеры.
    *   `routers/web/`: Обработчики HTML-страниц (UI).
    *   `routers/api/`: Обработчики REST API (v1).
    *   `routers/private/`: Внутренний API для взаимодействия с git-хуками и раннерами.
*   **`models/`**: **Данные (M)**. Структуры БД (Xorm beans) и базовые запросы.
    *   `models/db/`: Базовый движок и контекст БД.
    *   `models/user/`, `models/repo/`, `models/issue/`: Доменные модели.
*   **`services/`**: **Бизнес-логика**. "Толстые" сервисы, выполняющие сложные операции (создание репозитория с инициализацией git, отправка уведомлений и т.д.).
    *   *Правило*: Роутеры вызывают Сервисы, Сервисы вызывают Модели.
*   **`modules/`**: Утилиты и библиотеки (Middleware, Git-обертки, Логгер, Шаблонизатор).
    *   `modules/git/`: Взаимодействие с git-командой.
    *   `modules/repository/`: Утилиты для работы с репозиториями, включая `InternalPushingEnvironment`.
*   **`templates/`**: **Представление (V)**. Go-шаблоны (`.tmpl`), использующие синтаксис `{{...}}`.

### 1.2 Жизненный цикл запроса (Request Lifecycle)

Когда приходит HTTP-запрос (например, `GET /user/repo`):

1.  **Chi Router (`routers/routes.go`)**: Маршрутизатор определяет, какой хендлер вызвать.
2.  **Middleware (`services/context/`)**:
    *   Оборачивает `http.Request` в `context.Context` (свой, "толстый" контекст Forgejo).
    *   Загружает текущего пользователя (`ctx.Doer`).
    *   Загружает репозиторий, если URL содержит `/user/repo` (`ctx.Repo`).
3.  **Handler (`routers/web/...`)**:
    *   Получает данные из `ctx`.
    *   Выполняет проверки прав.
    *   Вызывает `services` или `models` для получения данных.
    *   Кладет данные в `ctx.Data["Key"] = Value`.
    *   Вызывает рендер: `ctx.HTML(200, "template_name")`.
4.  **Template**: Генерирует HTML, используя данные из `ctx.Data`.

---

## Часть 2: Образовательное расширение (Edu Extension)

Мы внедрили полноценный LMS-модуль ("LMS внутри Git") прямо в монолит Forgejo. Весь код расширения изолирован в отдельных директориях, чтобы минимизировать влияние на ядро.

### 2.1 Где лежит код?

| Слой | Путь | Назначение |
|------|------|------------|
| Модели + Сервисы + DAL | `internal/edu/` | Ядро бизнес-логики, интерфейсы, данные |
| HTTP-хендлеры | `routers/web/edu/` | Привязка к URL, обработка запросов |
| Middleware | `routers/web/edu/middleware.go` | Авторизация (reqEduTeacher, reqEduAdmin) |
| Шаблоны | `templates/edu/` | UI (Fomantic UI) |
| Интеграционные тесты | `tests/integration/edu_assignments_test.go` | SQLite-based Go тесты (15 тестов) |
| E2E тесты | `tests/e2e/edu.test.e2e.ts` | Playwright тесты (7 тестов) |
| Фикстуры | `models/fixtures/edu_user_role.yml` | Тестовые данные для edu ролей |
| Docker Dev | `edu-docker/Dockerfile.dev`, `edu-docker/docker-compose.yml`, `edu-docker/app.ini` | Локальная среда разработки |
| Документация | `edu-docs/` | Руководства для разработчиков и пользователей |

### 2.2 Архитектура модуля

Модуль следует чистой слоёной архитектуре. Сервис создаётся **один раз** при старте (`Init()`) и хранится как синглтон в `globalService`. Хендлеры получают его через `edu.GetService()`:

```
HTTP Handler (routers/web/edu/)
    │  вызывает edu.GetService()
    ▼
EducationalService (singleton, interface в service.go)
    │
    ├──> Repository (interface в service.go, реализация в repository_*.go)
    │        └──> Xorm ORM через db.GetEngine(ctx)
    │
    ├──> RepoForker (interface в service.go)
    │        └──> ForgejoAdapter (adapter.go) → Forgejo core services
    │
    ├──> UserCreator (interface в service.go)
    │        └──> ForgejoAdapter (adapter.go) → user_model.*
    │
    └──> OrgManager (interface в service.go)
             └──> ForgejoAdapter (adapter.go) → models.NewTeam, AddTeamMember, RemoveTeamMember
```

**Ключевые абстракции:**
- **`EducationalService`** — интерфейс бизнес-логики (~30 методов)
- **`Repository`** — интерфейс DAL (~40 методов для CRUD всех сущностей)
- **`RepoForker`** — абстракция над git-операциями (fork, sync, получение репо)
- **`UserCreator`** — абстракция над управлением пользователями
- **`OrgManager`** — абстракция над управлением командами организаций (EnsureTeam, AddTeamMember, RemoveTeamMember, GetTeam)
- **`ForgejoAdapter`** — мост к ядру Forgejo, реализует `RepoForker`, `UserCreator` и `OrgManager`

Все интерфейсы определены в `internal/edu/service.go`.

### 2.3 Модель данных (11 таблиц)

Все таблицы создаются Xorm-ом автоматически на старте через `e.Sync(...)` в `init.go`. Префикс `edu_` гарантируется методами `TableName()`. Ручных миграций для edu-таблиц нет — продакшена нет, при изменении схемы база пересоздаётся.

#### Основные сущности

| Таблица | Назначение | Ключевые поля |
|---|---|---|
| `edu_courses` | Курс | `Name`, `CreatorID`, `OrgID`, `TasksMasterRepoID` (опционально, ссылается на единственный `tasks-master`-репо курса), `StartUnix`, `EndUnix` |
| `edu_course_enrollments` | Зачисление | `CourseID`, `UserID`, `Role`, `GroupName` (опц., поток/группа), `StudentForkRepoID` (заполняется на стадии init-forks), UNIQUE(CourseID, UserID) |
| `edu_assignments` | Задание | `CourseID`, `TaskName` (имя папки в `tasks/` и часть имени ветки `submits/<TaskName>`), `AllowedFilesGlob` (авторитетный список путей, на которые студент имеет право), `Title`, `DeadlineUnix`, UNIQUE(CourseID, TaskName) |
| `edu_submissions` | Сабмит | `AssignmentID`, `EnrollmentID`, `BranchName` (= `submits/<TaskName>`), `PullRequestID` (0 пока notifier не создал PR), `Status`, `Grade`, `ManualGrade` (TA подтвердил — auto-grade больше не перезаписывает), `Comment`, `GradedByID`, UNIQUE(EnrollmentID, AssignmentID) |
| `edu_test_results` | Лог CI-прогона | `SubmissionID`, `CommitSHA`, `Score`, `Details` |
| `edu_user_role` | Глобальная роль | `UserID` UNIQUE, `Role` |

#### Импорт из CSV

| Таблица | Назначение |
|---|---|
| `edu_import_draft` | Сессия импорта (`CourseID`, `Status`, `RawCSV`) |
| `edu_import_draft_row` | Строка драфта (`FullName`, `Email`, `Group`, `Username`, `Role`, `Status`) |

#### Async-таски

| Таблица | Назначение |
|---|---|
| `edu_init_forks_task` | Инициализация форков `tasks-master` для всех зачисленных студентов (`CourseID`, `TotalUsers`, `Completed`, `Failed`, `Status`) |
| `edu_distribute_task` | Раздача ветки `submits/<TaskName>` всем студенческим форкам (`AssignmentID`, `TotalEnrollments`, `Pushed`, `Failed`, `Status`) |
| `edu_course_sync_task` | Запуск course-sync (`CourseID`, `TotalRepos`, `Synced`, `Skipped`, `Failed`, `Status`) |
| `edu_course_sync_pr` | Per-fork PR-запись для course-sync (`SyncTaskID`, `EnrollmentID`, `PullRequestID`, `Status`: pending/merged/conflict/failed) |

#### Жизненный цикл `Submission.Status`

```
pending  → ветка раздана, студент ещё не пушил
running  → CI идёт
done     → CI прошёл, PR создан, ждёт TA
approved → TA нажал Approve, оценка зафиксирована
merged   → TA нажал Merge
failed   → constraint check / тесты упали
```

#### Общие константы статусов (для async-task / draft)

В `models.go` определены `StatusDraft`, `StatusPending`, `StatusRunning`, `StatusDone`, `StatusError`.

### 2.4 Файлы `internal/edu/`

| Файл | Назначение |
|---|---|
| `models.go` | Xorm-структуры всех 11 таблиц + общие константы статусов |
| `init.go` | `Init()` — sync схемы, создание singleton-сервиса, регистрация notifier, загрузка локалей |
| `service.go` | Интерфейс `EducationalService` и фабрика `NewService` |
| `service_courses.go` | CRUD-операции для курсов и зачислений |
| `service_assignments_test.go`, `service_courses_test.go` | Unit-тесты на моках |
| `service_init_forks.go` | Инициализация форков (collaborator + branch protection) |
| `service_distribute.go` | Bulk push ветки `submits/<TaskName>` + создание `Submission(pending)` |
| `service_course_sync.go` | course-sync: push `course-sync`, открыть PR, auto-merge |
| `service_grading.go` | Approve / Merge / Comment / Reset-approval от имени `eduadmin` |
| `service_import.go` | CSV-imporт (upload → preview → execute), пробрасывание GroupName |
| `service_bulk_fork.go` | Внутренний хелпер для bulk-операций над форками (init-forks использует) |
| `service_ext.go` | Расширения сервиса для ad-hoc-вызовов из хендлеров |
| `csv_import.go` | Чистый парсер CSV (BOM, кодировки, разделители, транслитерация ГОСТ 7.79-2000) |
| `repository.go` + `repository_*.go` | Xorm-DAL: `repository_courses.go`, `repository_enrollments.go`, `repository_submissions.go`, `repository_test_results.go`, `repository_import.go`, `repository_init_forks.go`, `repository_distribute.go`, `repository_course_sync.go`, `repository_sync_pr.go`, `repository_ext.go` |
| `adapter.go` | `ForgejoAdapter`: bridge к ядру (forking, user creation, org teams) |
| `adapter_pulls.go` | Адаптер PR-операций: `NewPullRequest`, `Merge`, `CreateIssueComment`, чтение комментариев и diff-а |
| `adapter_actions.go` | Адаптер для `actions.ReadLogs` |
| `notifier.go` | Хук на `ActionRunNowDone` — constraint check, парсинг `::edu-grade::`, авто-PR, обновление статуса |
| `notifier_format_test.go`, `notifier_test.go` | Unit-тесты notifier-а |
| `grade_parser.go` | `ParseGradeFromLogLines` — извлекает `::edu-grade::XX` из текста логов |
| `role.go` | DAL для глобальной `UserRole` |
| `locale/locale_en-US.json`, `locale/locale_ru-RU.json` | Локали (~135+ ключей edu.*) |

### 2.5 Ключевые сценарии

#### А. Управление курсами

`POST /edu/teacher/courses/new` — handler `NewCoursePost`:
1. Валидация формы: `Name` (≤255), `Description`, `OrgID` (опц., dropdown орг, в которых юзер может создавать репо), `TasksMasterRepoID` (опц., dropdown репо в выбранной org), `StartUnix`/`EndUnix`.
2. `service.CreateCourse(...)` пишет `Course` + auto-зачисляет создателя как teacher через `EnrollUser(role=teacher)`. Если `OrgID` есть, добавляет его в команду `edu-course-{id}-teachers`.
3. Redirect на `/edu/teacher/courses/{id}`.

Удаление (`POST /{id}/delete`) каскадно удаляет 10 наборов данных в одной `db.WithTx`-транзакции: assignments, submissions, test_results, init_forks_task, distribute_tasks, course_sync_tasks + course_sync_pr, import_drafts + import_draft_rows, enrollments. Форки студентов остаются в org.

#### Б. CSV-импорт студентов

`POST /edu/teacher/courses/{id}/import` (3-этапный flow):
1. **Upload**: `service.UploadCSV(courseID, file, mapping)` парсит CSV (BOM, UTF-8/Win-1251, `,`/`;`, транслитерация ГОСТ 7.79-2000), создаёт `ImportDraft` + `ImportDraftRow` на каждую строку. `mapping.GroupCol` — индекс колонки группы (преподаватель указывает в форме upload).
2. **Preview** (`/import/{draftID}/preview`): таблица; редактирование через `/update-row` (`username`, `email`, `role`, `group_name`).
3. **Execute** (`/import/{draftID}/execute`): для каждой строки:
   - Если пользователь не существует → создать через `adapter.CreateUser(...)` (генерация пароля, `must_change_password=false`).
   - `service.EnrollUser(courseID, userID, role, GroupName=row.Group)`.
   - Если у курса `OrgID` — добавить в команду по роли.

#### В. Создание задания

Задание привязано к курсу 1:1 через `(CourseID, TaskName)` UNIQUE. Самого «репо задания» нет — папка `tasks/<TaskName>/` живёт внутри `Course.TasksMasterRepoID`.

`POST /edu/teacher/assignments/new` — handler `NewAssignmentPost`:
1. Валидация: `course_id` (required), `task_name` (regex `^[a-z0-9_-]+$`, ≤100), `title` (≤255), `allowed_files_glob` (required, ≤500), `deadline_unix`.
2. Проверка существования ветки: handler через адаптер вызывает `git.GetBranch(tasks-master, "submits/<TaskName>")`. Если ветки нет — flash error «Branch submits/<task_name> not found in tasks-master», форма не сохраняется.
3. `service.CreateAssignment(...)` пишет `Assignment` (без `RepoID`).

#### Г. Init forks (инициализация студенческих форков на уровне курса)

Заменяет старый «bulk fork на каждое задание». Один раз на курс — после CSV-импорта или ручного зачисления.

`POST /edu/teacher/courses/{id}/init-forks` — handler `InitForksPost`:
1. Валидация: курс активен, есть `TasksMasterRepoID`, есть незачисленные.
2. `service.StartInitForks(courseID)` создаёт `InitForksTask{Status=pending}`.
3. `go graceful.GetManager().RunWithShutdownContext(...)` — фоновая горутина:
   - Для каждой `Enrollment` без `StudentForkRepoID`:
     - `adapter.ForkRepository(student, tasksMasterRepo, name="<username>-tasks", target=Course.OrgID)` → `studentRepo`.
     - `adapter.AddCollaborator(studentRepo, student, accessMode=Write)`.
     - `adapter.ProtectMainBranch(studentRepo)` — `enable_push=false`, merge только команде teachers.
     - `repo.UpdateEnrollmentStudentForkRepoID(enrollmentID, studentRepo.ID)`.
     - Инкремент `task.Completed` (или `Failed` + добавить в `ErrorLog`).
   - В конце: `task.Status = done`.
4. Handler сразу redirect-ит на страницу курса.

Frontend опрашивает `/init-forks-status` (JSON: `{total, completed, failed, status}`).

#### Д. Distribute (раздача задания)

Заменяет старый «bulk fork на ассайнмент».

`POST /edu/teacher/assignments/{id}/distribute` — handler `DistributePost`:
1. Валидация: `submits/<TaskName>` существует в `tasks-master`; курс активен.
2. `service.StartDistribute(assignmentID)` создаёт `DistributeTask{Status=pending}`.
3. Goroutine: для каждой `Enrollment` курса со `StudentForkRepoID`:
   - `git.Push(tasksMasterRepo, studentForkRepo, "submits/<TaskName>:submits/<TaskName>")` через `InternalPushingEnvironment`.
   - `service.UpsertSubmission(EnrollmentID, AssignmentID, BranchName="submits/<TaskName>", Status=pending)`.
   - Инкремент `task.Pushed`/`task.Failed`.

Frontend опрашивает `/distribute-status`.

#### Е. CI/CD: путь от push студента до PR

Workflow в `tasks-master` (`.forgejo/workflows/grade.yml`) запускается на push в `refs/heads/submits/**`. Рекомендуемый шаблон (см. `edu-docker/test-kit/template-tasks-master/`):

```yaml
on:
  push:
    branches: ['submits/**']
jobs:
  grade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - name: Constraint check (по конвенции)
        run: |
          TASK=${GITHUB_REF#refs/heads/submits/}
          DIFF=$(git diff --name-only main...HEAD | grep -v "^tasks/$TASK/" || true)
          if [ -n "$DIFF" ]; then echo "Forbidden files: $DIFF"; exit 1; fi
      - name: Lint / Build / Test
        run: |
          # ... студенческий код ...
      - name: Grade marker
        if: success()
        run: echo "::edu-grade::100"
```

Server-side `notifier.go` подписан на `ActionRunNowDone`. На завершение:
1. **Идентификация**: по `repo_id` найти `Enrollment(StudentForkRepoID=repo.ID)`. По имени ветки — `TaskName`. Найти `Assignment(CourseID, TaskName)`.
2. **Авторитетный constraint check**: через `services/gitdiff.GetDiffShortStat` или `git.GetDiffForFile` сравнить ветку с `main` форка, отфильтровать по `Assignment.AllowedFilesGlob`. Нарушение → `Submission.Status=failed`, системный комментарий в PR (если PR уже есть).
3. **Парсинг `::edu-grade::`**: `actions.ReadLogs(jobs)` + `ParseGradeFromLogLines(...)`. Записать `Submission.Grade` только если `ManualGrade=false`.
4. **Auto-PR**: если `Submission.PullRequestID == 0` → `services/pull.NewPullRequest`:
   - `head = submits/<TaskName>`, `base = main`, `repo = enrollment.StudentForkRepoID`.
   - Title = `"[<GroupName>] Submit: <TaskName>"` (без префикса, если `GroupName==""`). Хелпер `formatSubmissionPRTitle(taskName, groupName)`.
   - Body = `"Auto-submit for task '<TaskName>' in course <CourseName>."`.
   - Сохранить `pr.ID` в `Submission.PullRequestID`.
5. **Создать TestResult**.
6. **Обновить Status**: `running` → `done` (зелёный CI) | `failed` (красный или constraint).

#### Ж. Submission review (Approve → Merge через edu-UI)

`GET /edu/teacher/assignments/{id}/submissions/{subID}` — handler `SubmissionReview`:
- Загружает `Submission`, `Enrollment`, `Assignment`, `Course`, PR (если есть).
- Через `services/gitdiff` получает diff `head=submits/<TaskName>` vs `base=main` форка.
- Через прямой Xorm к `models/issues` — все комментарии PR.
- Шаблон `submission_review.tmpl` инклюдит `templates/repo/diff/...` и `templates/repo/issue/view_content/comments.tmpl` partial-ы (внутри edu-навигации).

`POST .../approve` (`ApproveSubmissionPost`): валидация `grade` ∈ [0,100], `comment` (≤10 000); `Submission.Status=approved`, `ManualGrade=true`, `Grade=...`.

`POST .../merge` (`MergeSubmissionPost`): только если `Status=approved`. `services/pull.Merge` от имени `eduadmin`. `Status=merged`.

`POST .../comment`: `services/issue/comments.CreateIssueComment` от TA.

`POST .../reset-approval`: `Status=approved` → `done`, `ManualGrade=false`.

#### З. Course sync (PR-based синхронизация шаблона в форки)

Заменяет старый «sync forks» (прямой push в `main`).

`GET /edu/teacher/courses/{id}/sync` — страница со списком прошлых run-ов и кнопкой «Запустить».

`POST /edu/teacher/courses/{id}/sync/start` — handler `StartCourseSyncPost`:
1. Создать `CourseSyncTask{Status=pending}`.
2. Goroutine: для каждой `Enrollment` со `StudentForkRepoID`:
   - `git.Push(tasksMaster, fork, "main:course-sync")` через `InternalPushingEnvironment`.
   - `services/pull.NewPullRequest(head="course-sync", base="main", repo=fork)`.
   - Попытка `services/pull.Merge` (стратегия по умолчанию).
   - Запись `CourseSyncPR{SyncTaskID, EnrollmentID, PullRequestID, Status}`:
     - `merged` если auto-merge получился.
     - `conflict` если merge отказал по конфликту.
     - `failed` если push/PR-create вообще упали.

`GET /edu/teacher/courses/{id}/sync/status` — JSON прогресса.

`POST /edu/teacher/courses/{id}/sync/{taskID}/merge-all` — merge всех `Status=conflict?` нет — pending/non-conflict-PR-ов скопом.

`POST /edu/teacher/courses/{id}/sync/{taskID}/merge/{prID}` — merge одного PR (TA вручную после ресолва конфликта).

Страница course-sync поддерживает фильтр `?group=<GroupName>` (мульти-выбор).

#### И. Auto-grade vs ManualGrade

CI пишет `::edu-grade::XX` → notifier пишет `Submission.Grade` **только если `ManualGrade=false`**. На стадии `approved` (TA нажал Approve) `ManualGrade` ставится в `true` — последующие CI-прогоны (например, после merge с `course-sync`) грейд больше не перетирают. `Reset Approval` обнуляет `ManualGrade` обратно в `false`.

#### К. Маппинг ролей на org

| Edu роль | Org Team | `IncludesAllRepositories` | Repos | AccessMode | + collaborator |
|---|---|---|---|---|---|
| `student` | `edu-course-{id}-students` | false | только `tasks-master` | Read | Write на свой `<username>-tasks` |
| `ta` | `edu-course-{id}-ta` | true | вся org | Read | — |
| `teacher` / `admin` | `edu-course-{id}-teachers` | true | вся org | Admin | — |

При unenroll: убираем из team + убираем collaborator-запись. Сам fork в org остаётся (orphaned).

#### Л. Каскадное удаление курса

`repository_courses.go::DeleteCourse(courseID)` в `db.WithTx`:
1. `course_sync_pr` → 2. `course_sync_task` → 3. `distribute_task` → 4. `init_forks_task` → 5. `test_results` → 6. `submissions` → 7. `assignments` → 8. `import_draft_row` → 9. `import_draft` → 10. `course_enrollments` → 11. `courses`.

Форки студентов в org **не удаляются** автоматически — преподаватель может зачистить руками через GUI Forgejo при необходимости.

#### М. Ограничение доступа по активности курса

`course.IsActive()` (= `EndUnix == 0 || EndUnix > now()`) проверяется в:
- `GetAssignmentsForUser` (студенческий список) — SQL `WHERE end_unix = 0 OR end_unix > now()`.
- `StartInitForks`, `StartDistribute`, `StartCourseSync` — отказывают по неактивному курсу.
- Student assignment detail handler — flash error + редирект.

### 2.6 Полная карта роутов

Все edu-роуты в `routers/web/edu/routes.go`. Файлы хендлеров: `dashboard.go`, `courses.go`, `assignments.go`, `instructor.go`, `grading.go`, `course_sync.go`, `import.go`, `admin.go`.

#### Студент

| Метод | Путь | Хендлер | Что делает |
|---|---|---|---|
| GET | `/edu/dashboard` | `Dashboard` | Redirect по роли |
| GET | `/edu/student/assignments` | `StudentAssignments` | Список своих сабмитов (только активные курсы) |
| GET | `/edu/student/assignments/{id}` | `AssignmentDetail` | Детали: ссылка на fork, ветку, CI, PR, оценку |

#### Преподаватель / TA

| Метод | Путь | Хендлер | Доступ |
|---|---|---|---|
| GET | `/edu/teacher/courses` | `CourseList` | TA / Teacher |
| GET / POST | `/edu/teacher/courses/new` | `NewCourse` / `NewCoursePost` | Teacher |
| GET | `/edu/teacher/courses/{id}` | `CourseDetail` | TA / Teacher |
| GET / POST | `/edu/teacher/courses/{id}/edit` | `EditCourse` / `EditCoursePost` | Teacher (creator) |
| POST | `/edu/teacher/courses/{id}/delete` | `DeleteCoursePost` | Teacher (creator) |
| POST | `/edu/teacher/courses/{id}/enroll` | `EnrollUserPost` | Teacher |
| POST | `/edu/teacher/courses/{id}/unenroll` | `RemoveEnrollmentPost` | Teacher |
| POST | `/edu/teacher/courses/{id}/init-forks` | `InitForksPost` | Teacher |
| GET | `/edu/teacher/courses/{id}/init-forks-status` | `InitForksStatus` | Teacher |
| GET | `/edu/teacher/courses/{id}/sync` | `CourseSyncPage` | TA / Teacher |
| POST | `/edu/teacher/courses/{id}/sync/start` | `StartCourseSyncPost` | Teacher |
| GET | `/edu/teacher/courses/{id}/sync/status` | `CourseSyncStatus` | TA / Teacher |
| POST | `/edu/teacher/courses/{id}/sync/{taskID}/merge-all` | `MergeAllCourseSyncPost` | TA / Teacher |
| POST | `/edu/teacher/courses/{id}/sync/{taskID}/merge/{prID}` | `MergeOneCourseSyncPost` | TA / Teacher |
| GET / POST | `/edu/teacher/courses/{id}/import` | `ImportUpload` / `ImportUploadPost` | Teacher |
| GET | `/edu/teacher/courses/{id}/import/{draftID}/preview` | `ImportPreview` | Teacher |
| POST | `/edu/teacher/courses/{id}/import/{draftID}/update-row` | `ImportUpdateRow` | Teacher |
| POST | `/edu/teacher/courses/{id}/import/{draftID}/execute` | `ImportExecutePost` | Teacher |
| POST | `/edu/teacher/courses/{id}/import/{draftID}/delete` | `ImportDeletePost` | Teacher |
| GET | `/edu/teacher/assignments` | `TeacherAssignments` | TA / Teacher |
| GET / POST | `/edu/teacher/assignments/new` | `NewAssignment` / `NewAssignmentPost` | Teacher |
| GET / POST | `/edu/teacher/assignments/{id}/edit` | `EditAssignment` / `EditAssignmentPost` | Teacher |
| POST | `/edu/teacher/assignments/{id}/delete` | `DeleteAssignmentPost` | Teacher |
| POST | `/edu/teacher/assignments/{id}/distribute` | `DistributePost` | Teacher |
| GET | `/edu/teacher/assignments/{id}/distribute-status` | `DistributeStatus` | TA / Teacher |
| GET | `/edu/teacher/assignments/{id}/submissions` | `InstructorSubmissions` | TA / Teacher |
| GET | `/edu/teacher/assignments/{id}/submissions/{subID}` | `SubmissionReview` | TA / Teacher |
| POST | `/edu/teacher/assignments/{id}/submissions/{subID}/approve` | `ApproveSubmissionPost` | TA / Teacher |
| POST | `/edu/teacher/assignments/{id}/submissions/{subID}/merge` | `MergeSubmissionPost` | TA / Teacher |
| POST | `/edu/teacher/assignments/{id}/submissions/{subID}/comment` | `CommentSubmissionPost` | TA / Teacher |
| POST | `/edu/teacher/assignments/{id}/submissions/{subID}/reset-approval` | `ResetApprovalPost` | TA / Teacher |
| GET | `/edu/teacher/dashboard` | redirect | TA / Teacher |

«Teacher» = full teacher (`isFullTeacher` — teacher / admin / site admin). «Teacher (creator)» = плюс `course.CreatorID == ctx.Doer.ID`. Удалённые роуты: `/edu/student/assignments/{id}/join`, `/edu/teacher/assignments/{id}/bulk-fork(-status)`, `/edu/teacher/assignments/{id}/sync-forks(-status)` — заменены на init-forks / distribute / course-sync уровня курса.

#### Админ

| Метод | Путь | Хендлер |
|---|---|---|
| GET | `/edu/admin` | `AdminPanel` |
| POST | `/edu/admin/roles` | `UpdateUserRolePost` |

### 2.7 Интеграция с ядром Forgejo

Edu-модуль затрагивает ядро Forgejo в **4 точках**:

| Точка | Файл ядра | Что делается |
|-------|-----------|-------------|
| **Init** | `routers/init.go` | `mustInitCtx(ctx, edu.Init)` — запуск инициализации (sync схемы, создание singleton-сервиса, регистрация нотификатора, загрузка локалей) |
| **Routes** | `routers/web/web.go` | `edu.RegisterRoutes(m, ...)` — регистрация роутов |
| **Notifier** | (runtime) | `notify.RegisterNotifier(&EduNotifier{})` — подписка на события |
| **Navbar** | `templates/custom/extra_links.tmpl` | Ссылка "Education" в навигации |

Весь остальной код изолирован в `internal/edu/`, `routers/web/edu/`, `templates/edu/`.

---

## Часть 3: Технический стек и паттерны

### 3.1 ORM: Xorm

Edu-модуль использует **Xorm** — тот же ORM, что и ядро Forgejo. Все запросы идут через `db.GetEngine(ctx)`:

```go
// Insert
_, err := db.GetEngine(ctx).Insert(assignment)

// Select by ID
has, err := db.GetEngine(ctx).ID(id).Get(assignment)

// Select with conditions
err := db.GetEngine(ctx).Where("course_id = ?", courseID).OrderBy("created_unix DESC").Find(&assignments)

// Update specific columns
_, err := db.GetEngine(ctx).ID(a.ID).Cols("title", "description", "deadline_unix", "updated_unix").Update(a)

// Join
err := db.GetEngine(ctx).
    Join("INNER", "edu_course_enrollments", "edu_course_enrollments.course_id = edu_assignments.course_id").
    Where("edu_course_enrollments.user_id = ?", userID).
    Find(&assignments)
```

Xorm используется как для auto-migration схемы (`e.Sync()`), так и для всех CRUD-операций. Это обеспечивает единообразие с ядром Forgejo и отсутствие внешних зависимостей (squirrel ранее использовался, но был заменён).

### 3.2 Фронтенд: Server-side rendering

- **Fomantic UI** (форк Semantic UI) — CSS-фреймворк. Классы: `ui container`, `ui segment`, `ui celled table`, `ui label`, `ui form`.
- **jQuery** — минимально, для interactive компонентов.
- **Go templates** — `{{.Variable}}`, `{{range}}`, `{{if}}`, `{{template "base/head"}}`.
- **Встроенные хелперы**: `{{svg "octicon-check"}}` для иконок, `{{.CsrfTokenHtml}}` для CSRF, `{{(DateUtils).AbsoluteShort .Timestamp}}` для дат.
- **Приведение типов**: Go templates **не имеют** функции `string`. Для приведения кастомных типов (например, `SubmissionStatus`, `RoleType`) к строке используйте `{{printf "%s" .Value}}`.
- **Нет SPA**: каждая страница рендерится сервером.

### 3.3 Локализация (i18n)

Forgejo использует собственную систему интернационализации на основе INI/JSON-файлов.

#### Где хранятся переводы

Edu-переводы хранятся **отдельно от ядра Forgejo** в JSON-файлах, встроенных через Go embed:

```
internal/edu/locale/
├── locale_en-US.json   — английский
└── locale_ru-RU.json   — русский
```

Формат — плоский JSON с полными ключами:

```json
{
    "edu.assignments": "Задания",
    "edu.new_assignment": "Новое задание",
    "edu.no_assignments": "Заданий не найдено.",
    "edu.roles": "Роли",
    "edu.manage_roles": "Управление ролями"
}
```

При инициализации (`init.go`) файлы загружаются через `i18n.DefaultLocales.AddToLocaleFromJSON()` и мерджатся с основными переводами Forgejo. Это позволяет **не трогать** core locale файлы (`options/locale/*.ini`).

#### Использование в шаблонах

В Forgejo шаблонах локаль доступна через template function `ctx`, а **не** через `.locale`:

```html
<!-- ПРАВИЛЬНО — ctx это template function, возвращает templates.Context -->
{{ctx.Locale.Tr "edu.roles"}}

<!-- НЕПРАВИЛЬНО — .locale ищет ctx.Data["locale"], которого не существует -->
{{.locale.Tr "edu.roles"}}
```

`ctx.Locale.Tr` работает одинаково как внутри `{{range}}`, так и вне — потому что `ctx` это функция, а не поле данных. Внутри `{{range}}` точка (`.`) меняется на текущий элемент, но `ctx` по-прежнему доступен:

```html
{{range .Assignments}}
    <!-- . = текущий Assignment, но ctx.Locale всё ещё работает -->
    <span>{{ctx.Locale.Tr "edu.view"}}</span>
{{end}}
```

#### Как добавить новый ключ перевода

1. Добавить ключ в `internal/edu/locale/locale_en-US.json`:
   ```json
   "edu.my_new_key": "My new text"
   ```
2. Добавить перевод в `internal/edu/locale/locale_ru-RU.json`:
   ```json
   "edu.my_new_key": "Мой новый текст"
   ```
3. Использовать в шаблоне: `{{ctx.Locale.Tr "edu.my_new_key"}}`
4. Использовать в Go-хендлере: `ctx.Tr("edu.my_new_key")`

#### Как это работает под капотом

1. При старте Forgejo `modules/translation/translation.go` → `InitLocales()` загружает core INI-файлы.
2. Затем `internal/edu/init.go` → `Init()` загружает edu JSON-файлы через `i18n.DefaultLocales.AddToLocaleFromJSON()`.
3. JSON-ключи попадают в `newStyleMessages` map, которая проверяется **первой** при вызове `TrString()`.
4. Если ключ не найден — fallback на дефолтный язык (en-US). Если и там нет — возвращается имя ключа.
5. В шаблонах `ctx` — template function из `htmlrenderer.go`, возвращает `templates.Context` с полем `Locale`.

### 3.4 Локальная разработка (Docker)

Forgejo не компилируется на Windows (файлы `_unix.go` используют `syscall.Setpgid`, `unix.Umask` и т.д.). Для локальной разработки используется Docker.

**Файлы:**
- `edu-docker/Dockerfile.dev` — двухстадийная сборка: `golang:1.25-alpine` (build) → `alpine:3.23` (runtime). Запуск от пользователя `git` (Forgejo не работает от root).
- `edu-docker/docker-compose.yml` — сервисы: `forgejo` (порт 3000), `forgejo-runner` (CI/CD runner с авторегистрацией), опциональные `test-integration`, `test-unit`, `test-e2e`.
- `edu-docker/Dockerfile.test` — сборка для запуска integration и unit тестов.
- `edu-docker/Dockerfile.e2e` — сборка для запуска E2E (Playwright) тестов.
- `edu-docker/app.ini` — преконфигурированный SQLite, `INSTALL_LOCK=true` (пропускает Install Wizard), Forgejo Actions включены.

**Запуск:**
```bash
cd forgejo-edu
docker compose -f edu-docker/docker-compose.yml build forgejo
docker compose -f edu-docker/docker-compose.yml up forgejo
# Сайт: http://localhost:3000, первый зарегистрированный пользователь — админ
```

**Сброс БД:**
```bash
docker compose -f edu-docker/docker-compose.yml down -v   # -v удаляет volume с данными
docker compose -f edu-docker/docker-compose.yml up forgejo --build
```

### 3.5 Тестирование

| Тип | Файлы | Как запускать |
|-----|-------|--------------|
| Unit-тесты (Go) | `internal/edu/*_test.go` (13 файлов, ~61 тестовая функция) | `go test ./internal/edu/...` |
| Integration (Go + SQLite) | `tests/integration/edu_assignments_test.go` (15 тестов) | `make test-sqlite#TestEdu` |
| E2E (Playwright) | `tests/e2e/edu.test.e2e.ts` (7 тестов) | `make test-e2e-sqlite` |

**Integration тесты**: in-memory SQLite, fixtures из `models/fixtures/*.yml` (включая `edu_user_role.yml` для edu ролей), хелперы `tests.PrepareTestEnv(t)()` и `loginUser(t, "user1")`. Тесты используют helper `ensureEduTables()` для создания edu-таблиц через DDL и `setupEduEnv()` для подготовки окружения.

**E2E тесты**: Playwright, SQLite-сервер на `localhost:3003`, fixture-юзеры (password="password").

**Docker-тесты:**
```bash
docker compose -f edu-docker/docker-compose.yml run --rm test-integration   # Integration
docker compose -f edu-docker/docker-compose.yml run --rm test-unit          # Unit
```

### 3.6 Ошибки и логирование

```go
// Паттерн ошибок
return fmt.Errorf("context: %w", err)

// Логирование
log.Error("EduNotifier: failed to create test result: %v", err)
log.Info("Educational Extension initialized successfully.")
```

### 3.7 Права доступа и авторизация

#### Middleware (роутерный уровень)
- `reqEduTeacher` — пропускает только пользователей с edu-ролью `ta`, `teacher`, или `admin` (или site admin). Все `/edu/teacher/*`-роуты под ним.
- `reqEduAdmin` — пропускает только site admin или edu admin. `/edu/admin/*`.
- Проверка `isFullTeacher` (teacher / admin / site admin) — внутри хендлеров для мутирующих операций (CRUD, init-forks, distribute, course-sync start, import, enrollment).

#### Проверка владения курсом (handler-уровень)
Любая мутация курса (`/edit`, `/delete`, `/enroll`, `/unenroll`, `/init-forks`, `/import/*`) проверяет `course.CreatorID == ctx.Doer.ID` (site admin может всё). Это защищает от кросс-преподавательской подделки `course_id`.

#### Права в Git-слое
- **Student**: член team `edu-course-{id}-students` (Read на `tasks-master` через `IncludesAllRepositories=false` + repo-список). На свой `<username>-tasks` — Write через collaborator-запись. На чужие форки — никаких прав. Push в свой `main` ему запрещён branch-protection-ом; merge только через edu-UI под `eduadmin`.
- **TA**: член team `edu-course-{id}-ta` (`IncludesAllRepositories=true`, Read на всю org). Видит `tasks-master` и все форки read-only. Через edu-UI — может комментировать, Approve, Merge.
- **Teacher / Admin**: член team `edu-course-{id}-teachers` (Admin на всю org). Может всё.

#### Системный пользователь `eduadmin`
Все merge-операции (Approve→Merge, course-sync auto-merge, ручной merge через UI) выполняются от имени `eduadmin` через прямые internal Go-вызовы. Это тот же пользователь, который используется для регистрации `forgejo-runner`. У него — site admin плюс ownership на org-и созданные при первом запуске.

### 3.8 Обработка ошибок

Все операции с побочными эффектами (обновление BulkForkTask, SyncForkTask, ImportDraftRow, ImportDraft) логируют ошибки через `log.Error(...)` вместо игнорирования. Паттерн:

```go
if err := s.repo.UpdateBulkForkTask(ctx, task); err != nil {
    log.Error("failed to update bulk fork task: %v", err)
}
```

### 3.9 XSS-защита в шаблонах

Все пользовательские данные в шаблонах пропускаются через `| SanitizeHTML`. **Никогда не используйте `| SafeHTML`** для данных, введённых пользователем (названия курсов, описания, комментарии к оценкам и т.д.):

```html
<!-- ПРАВИЛЬНО -->
<p>{{.Course.Description | SanitizeHTML}}</p>

<!-- НЕПРАВИЛЬНО — XSS уязвимость -->
<p>{{.Course.Description | SafeHTML}}</p>
```

---

## Часть 4: Как развивать проект

### Добавление новой сущности

1. Определить структуру в `models.go` (с тегами `xorm` и `json`). Добавить метод `TableName()` для задания имени таблицы с `edu_` префиксом.
2. Добавить `new(Entity)` в `e.Sync()` в `init.go`.
3. Добавить CRUD-методы в интерфейс `Repository` (`service.go`).
4. Реализовать методы в `repository_<entity>.go`.
5. Добавить бизнес-методы в `EducationalService` и реализовать в `service_<entity>.go`.
6. Создать хендлеры в `routers/web/edu/<entity>.go`.
7. Добавить роуты в `routes.go`.
8. Создать шаблоны в `templates/edu/<entity>_*.tmpl`.
9. Написать тесты.

### Добавление нового роута

1. Добавить строку в `routes.go` внутри нужной группы (`/student`, `/teacher`, `/admin`).
2. Создать хендлер в соответствующем файле.
3. Паттерн хендлера:
   ```go
   func MyHandler(ctx *context.Context) {
       svc := edu.GetService()  // singleton, НЕ создавать новый сервис
       // ... бизнес-логика ...
       ctx.Data["Key"] = value
       ctx.HTML(http.StatusOK, tplMyTemplate)
   }
   ```

### Шпаргалка

*   **Как добавить роут?** → `routers/web/edu/routes.go`
*   **Где модели?** → `internal/edu/models.go`
*   **Где логика?** → `internal/edu/service_*.go`
*   **Где SQL?** → `internal/edu/repository_*.go`
*   **Где шаблоны?** → `templates/edu/`
*   **Где мост к Forgejo?** → `internal/edu/adapter.go`
*   **Как зарегистрировать роуты в ядре?** → `routers/web/web.go` → `edu.RegisterRoutes`
*   **Как подписаться на события?** → `internal/edu/notifier.go` → `notify.RegisterNotifier`

Код модульный: изменения в `internal/edu/` не влияют на ядро Forgejo. Удачи!

---

## Часть 5: Обновление из upstream Forgejo

Образовательное расширение — это форк Forgejo. Периодически нужно подтягивать изменения из основного репозитория. Благодаря минимальным вмешательствам в ядро (всего 4 строки в 2 файлах), конфликты при мерже будут редкими и тривиальными.

### 5.1 Точки вмешательства в ядро

| Файл | Изменение | Строки |
|------|-----------|--------|
| `routers/init.go` | Импорт `"forgejo.org/internal/edu"` | +1 строка в `import` |
| `routers/init.go` | `mustInitCtx(ctx, edu.Init)` | +1 строка после `repo_service.Init` |
| `routers/web/web.go` | Импорт `"forgejo.org/routers/web/edu"` | +1 строка в `import` |
| `routers/web/web.go` | `edu.RegisterRoutes(m, reqSignIn)` | +1 строка перед `// ***** START: User *****` |

Всё остальное (модели, сервисы, роутеры, шаблоны, локали) живёт в изолированных директориях (`internal/edu/`, `routers/web/edu/`, `templates/edu/`) и не конфликтует с upstream.

### 5.2 Автоматический мерж (рекомендуется)

В корне репозитория есть скрипт `edu_merge_upstream.sh`, который автоматизирует весь процесс:

```bash
./edu_merge_upstream.sh           # мерж из upstream/forgejo
./edu_merge_upstream.sh v12.0     # мерж из конкретной ветки
```

Скрипт автоматически:
- Добавляет upstream remote (если нет)
- Создаёт ветку `merge-upstream-YYYY-MM-DD`
- Выполняет мерж
- Резолвит `go.sum`/`go.mod` конфликты через `go mod tidy`
- Проверяет что edu-строки в ядре не потеряны
- Если есть сложные конфликты — останавливается и просит ручной помощи

### 5.3 Ручная инструкция

Если скрипт недоступен или нужен полный контроль:

```bash
# 1. Добавить upstream remote (один раз)
git remote add upstream https://codeberg.org/forgejo/forgejo.git

# 2. Получить свежие изменения
git fetch upstream

# 3. Создать ветку для мержа
git checkout forgejo
git checkout -b merge-upstream-YYYY-MM-DD

# 4. Выполнить мерж
git merge upstream/forgejo

# 5. Разрешить конфликты (см. ниже)

# 6. Проверить сборку
make build

# 7. Запустить тесты
make test-sqlite#TestEdu

# 8. Создать PR для review
```

### 5.4 Типичные конфликты и их решение

#### `go.sum` (будет конфликтовать почти всегда)
Это автогенерируемый файл. Резолвить руками не нужно:
```bash
git checkout --theirs go.sum
go mod tidy
git add go.sum
```

#### `go.mod` (редко)
Если upstream обновил Go-зависимости:
```bash
# Принять upstream версию, затем добавить наши зависимости обратно (если есть)
git checkout --theirs go.mod
go mod tidy
git add go.mod
```

#### `routers/web/web.go` (редко)
Upstream может добавить новые роуты рядом с нашей точкой вставки. Нужно принять обе стороны:
1. Оставить все upstream изменения
2. Убедиться что `"forgejo.org/routers/web/edu"` есть в `import`
3. Убедиться что `edu.RegisterRoutes(m, reqSignIn)` стоит перед `// ***** START: User *****`
```bash
# После ручной правки:
git add routers/web/web.go
```

#### `routers/init.go` (очень редко)
Аналогично web.go — убедиться что наши 2 строки на месте:
- `"forgejo.org/internal/edu"` в импорте
- `mustInitCtx(ctx, edu.Init)` после `repo_service.Init`

### 5.5 Верификация после мержа

```bash
# Сборка
make build

# Edu-тесты
make test-sqlite#TestEdu

# Юнит-тесты edu
go test ./internal/edu/...

# Полный тест-сьют (опционально, долго)
make test
```

### 5.6 Деплой после мержа

Время простоя при обновлении — **минимальное** (1–5 минут):

1. **Сборка** (`make build`) — 3–10 минут (зависит от сервера)
2. **Остановка сервиса** → **Запуск новой версии** — секунды
3. **Миграция БД** — автоматическая. Xorm `Sync()` при старте добавит новые колонки/таблицы. Edu-таблицы мигрируются через `edu.Init()`.

Данные не теряются. Откат — запустить предыдущий бинарник.

```bash
# Типичный деплой:
cd /path/to/forgejo-edu
git pull
make build
sudo systemctl restart forgejo
# Готово. Forgejo поднимется за ~5 секунд.
```

### 5.7 Рекомендации

- **Мержить регулярно** (раз в 1–2 месяца). Чем реже — тем больше конфликтов.
- **Не модифицировать файлы ядра** без крайней необходимости. Все новые фичи — в `internal/edu/`, `routers/web/edu/`, `templates/edu/`.
- **Использовать extension points Forgejo** где возможно (`templates/custom/`, `notify.RegisterNotifier`, `i18n.AddToLocaleFromJSON`).
- **`go.sum` конфликты — нормально**. Это не проблема, а рутина. `go mod tidy` фиксит их за секунду.
