# Погружение в архитектуру Forgejo и Образовательного расширения

Этот документ предназначен для разработчика, желающего понять внутреннее устройство проекта "с нуля". Он охватывает архитектуру платформы Forgejo и детали реализации образовательного модуля (Edu Extension).

## Содержание

- [Часть 1: Архитектура Forgejo](#часть-1-архитектура-forgejo)
  - [1.1 Структура папок](#11-структура-папок)
  - [1.2 Жизненный цикл запроса](#12-жизненный-цикл-запроса-request-lifecycle)
- [Часть 2: Образовательное расширение](#часть-2-образовательное-расширение-edu-extension)
  - [2.1 Где лежит код?](#21-где-лежит-код)
  - [2.2 Архитектура модуля](#22-архитектура-модуля)
  - [2.3 Модель данных (10 таблиц)](#23-модель-данных-10-таблиц)
  - [2.4 Файлы internal/edu/](#24-файлы-internaledu)
  - [2.5 Ключевые сценарии](#25-ключевые-сценарии)
    - [А. Управление курсами](#а-управление-курсами)
    - [Б. CSV-импорт студентов](#б-csv-импорт-студентов)
    - [В. Создание задания](#в-создание-задания)
    - [Г. Студент берёт задание (Join)](#г-студент-берёт-задание-join)
    - [Д. Массовый форк (Bulk Fork)](#д-массовый-форк-bulk-fork)
    - [Е. Синхронизация форков (Sync Forks)](#е-синхронизация-форков-sync-forks)
    - [Ж. CI/CD интеграция](#ж-cicd-интеграция-автоматическое-тестирование)
    - [З. Ручное оценивание](#з-ручное-оценивание-grading)
    - [И. Административная панель](#и-административная-панель)
    - [К. Маппинг ролей на организацию](#к-маппинг-ролей-на-организацию-forgejo)
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

![Диаграмма вариантов использования](use-case-diagram.png)

#### А. Управление курсами

Файлы: `routers/web/edu/courses.go`, `templates/edu/course_list.tmpl`, `course_detail.tmpl`, `course_form.tmpl`

1. Преподаватель заходит на `/edu/teacher/courses` — видит список своих курсов.
2. Создаёт курс через `/edu/teacher/courses/new` (название, описание, даты).
3. На странице курса (`/edu/teacher/courses/{id}`) видит список участников и может:
   - Добавить студента вручную (по username)
   - Импортировать студентов из CSV
   - Удалить участника
4. Курс привязывается к заданиям — задание всегда принадлежит курсу.

#### Б. CSV-импорт студентов

![Диаграмма последовательности: CSV-импорт](seq-diagram-students-csv-import.png)

Файлы: `routers/web/edu/import.go`, `internal/edu/csv_import.go`, `internal/edu/translit.go`, `service_import.go`

Трёхшаговый процесс:

1. **Upload**: Преподаватель загружает CSV (поддерживает UTF-8, Windows-1251, BOM, запятая и точка с запятой как разделитель).
2. **Preview**: Система парсит CSV, транслитерирует ФИО в username (`Иванов Иван` → `ivanov-i`), показывает таблицу для редактирования (можно исправить username/email).
3. **Execute**: Система создаёт пользователей (с автогенерацией пароля), записывает их в курс. Показывает таблицу с логинами и паролями для раздачи.

Если пользователь уже существует — просто записывается в курс без создания нового.

#### В. Создание задания

Файлы: `routers/web/edu/assignments.go`, `templates/edu/assignment_new.tmpl`

1. Преподаватель создаёт обычный репозиторий-шаблон в Forgejo.
2. Заходит на `/edu/teacher/assignments/new`, выбирает курс и репозиторий, заполняет название, описание, дедлайн.
3. Запись создаётся в `edu_assignments` с привязкой к `course_id` и `repo_id`.

**Техническая реализация**: Хелпер `loadCoursesAndRepos()` в `assignments.go` загружает курсы и определяет, какие репозитории показать. Если курс привязан к организации, отображаются репозитории организации. Если у организации нет репозиториев, список остаётся пустым (нет fallback на собственные репозитории пользователя). Если курс без организации — показываются собственные репозитории пользователя. JavaScript на клиенте при смене курса делает redirect с `?course_id=X`, чтобы сервер подгрузил правильные репозитории.

#### Г. Студент берёт задание (Join)

![Диаграмма последовательности: Студент берёт задание](seq-diagram-student-assignment.png)

Файлы: `routers/web/edu/assignments.go` → `JoinAssignment`, `internal/edu/service_join.go`

1. Студент видит список своих заданий на `/edu/student/assignments` (только по курсам, в которых он записан).
2. Нажимает "Start Assignment" на `/edu/student/assignments/{id}`.
3. Сервис вызывает `RepoForker.ForkRepositoryAndUpdates` — создаёт форк шаблона в namespace студента.
4. Создаётся запись `Submission` со статусом `started` и ссылкой на форк.
5. Студент перенаправляется на страницу задания, где видит ссылку на свой репозиторий.

#### Д. Массовый форк (Bulk Fork)

![Диаграмма последовательности: Асинхронный массовый форк](seq-diagram-bulk-fork.png)

Файлы: `routers/web/edu/bulk_fork.go`, `internal/edu/service_bulk_fork.go`

1. На странице submissions преподаватель нажимает "Fork for All Students".
2. Система создаёт задачу `BulkForkTask` со статусом `pending` и запускает фоновую горутину через `graceful.GetManager().RunWithShutdownContext()`.
3. HTTP-запрос сразу возвращает redirect — страница не блокируется.
4. Горутина для каждого студента:
   - Проверяет, нет ли уже submission.
   - Создаёт форк шаблона → создаёт submission.
   - Обновляет прогресс в БД (completed/failed/total).
5. Фронтенд опрашивает `/bulk-fork-status` (JSON) для отображения прогресса.

#### Е. Синхронизация форков (Sync Forks)

Файлы: `routers/web/edu/sync_fork.go`, `internal/edu/service_sync_fork.go`, `adapter.go`

1. Преподаватель обновляет шаблон-репозиторий (добавляет тесты, README и т.д.).
2. Нажимает "Sync All Forks" на странице submissions.
3. Система создаёт задачу `SyncForkTask` со статусом `pending` и запускает фоновую горутину через `graceful.GetManager().RunWithShutdownContext()`.
4. HTTP-запрос сразу возвращает redirect — страница не блокируется.
5. Горутина для каждого форка студента выполняет `git push` из шаблона в форк, обновляя прогресс в БД.
6. Фронтенд опрашивает `/sync-fork-status` (JSON) для отображения прогресса.

**Ключевое решение**: используется `InternalPushingEnvironment` из `modules/repository/env.go`, что устанавливает переменную `GITEA_INTERNAL_PUSH=true`. Это приводит к тому, что git-хуки Forgejo пропускают проверку branch protection. Без этого `push` в чужой форк блокировался бы защитой веток.

```go
// adapter.go — ключевой фрагмент
func (a *ForgejoAdapter) SyncFork(ctx context.Context, doer *user_model.User, forkRepo *repo_model.Repository, branch string) error {
    return git.Push(ctx, forkRepo.BaseRepo.RepoPath(), git.PushOptions{
        Remote: forkRepo.RepoPath(),
        Branch: fmt.Sprintf("%s:%s", branch, branch),
        Env:    repo_module.InternalPushingEnvironment(doer, forkRepo),
    })
}
```

#### Ж. CI/CD интеграция (автоматическое тестирование)

![Диаграмма последовательности: Push → CI/CD → Результат](seq-diagram-student-submission.png)

Файл: `internal/edu/notifier.go`

1. `EduNotifier` регистрируется через `notify.RegisterNotifier` в `Init()` (не в `NewService()`).
2. Когда Forgejo Actions runner завершает workflow, срабатывает `ActionRunNowDone`.
3. Наш нотификатор:
   - Ищет submission по `run.RepoID` (это форк студента).
   - Если нашёл — обновляет статус: `Success` → `passed`, `Failure` → `failed`.
   - Парсит оценку из логов CI (см. ниже) и создаёт запись `TestResult` с `CommitSHA`, `Score` и описанием.

#### Ж.1. Автоматическое выставление оценки из CI-логов

Файлы: `internal/edu/notifier.go`, `internal/edu/grade_parser.go`

Workflow может вывести оценку в лог командой вида `::edu-grade::XX`, где `XX` — целое число от 0 до 100. Нотификатор разбирает эту команду и записывает оценку в `TestResult.Score`, а затем (если `submission.ManualGrade == false`) — в `submission.Grade`.

**Формат команды:**

```
::edu-grade::85
```

**Пример workflow:**

```yaml
- name: Run tests and grade
  run: |
    PASS=$(go test ./... 2>&1 | grep -c PASS || true)
    TOTAL=10
    SCORE=$(( PASS * 100 / TOTAL ))
    echo "::edu-grade::${SCORE}"
```

**Алгоритм парсинга (`ParseGradeFromLogLines` в `grade_parser.go`):**

- Регулярное выражение: `::edu-grade::(\d{1,3})`
- Разбираются все строки логов всех jobs и tasks workflow через `actions.ReadLogs`.
- Побеждает **последнее** вхождение команды.
- Значение валидируется в диапазоне 0–100; невалидные значения игнорируются.
- Если команда `::edu-grade::` не найдена в логах — применяется **бинарная оценка**: 100 при успехе workflow, 0 при неудаче (поведение до введения фичи).

**Поле `ManualGrade` на `Submission`:**

- `ManualGrade bool` (DEFAULT `false`) — флаг, что преподаватель вручную выставил оценку.
- Если `ManualGrade == true`, нотификатор **не перезаписывает** `submission.Grade` результатом CI.
- При сохранении оценки через форму (`POST .../grade`) сервис устанавливает `ManualGrade = true`.

**Сброс на автоматическую оценку (`ResetToAutoGrade`):**

- Сервисный метод `ResetToAutoGrade` устанавливает `ManualGrade = false` и восстанавливает `Grade` из последнего `TestResult.Score`.
- Роут: `POST /edu/teacher/assignments/{id}/submissions/{subID}/reset-grade`.
- На странице детали submission отображается кнопка **"Reset to auto-grade"**, которая вызывает этот роут.

#### З. Ручное оценивание (Grading)

Файлы: `routers/web/edu/grading.go`, `internal/edu/service_grading.go`, `templates/edu/submission_detail.tmpl`

1. На странице submissions (`/edu/teacher/assignments/{id}/submissions`) преподаватель видит таблицу со столбцами: Student, Status, CI Score, Grade, Repo, Updated, Detail.
2. Нажимает "Detail" → переходит на `/edu/teacher/assignments/{id}/submissions/{subID}`.
3. Видит: информацию о студенте, историю CI-прогонов (таблица TestResult), форму оценивания.
4. Вводит оценку (0–100) и комментарий, нажимает "Save Grade".
5. Оценка сохраняется в полях `grade`, `comment`, `graded_by_id`, `graded_unix` таблицы submissions, флаг `manual_grade` устанавливается в `true`.

Студент видит свою оценку на странице задания (`/edu/student/assignments/{id}`), а также последний результат CI.

#### И. Административная панель

Файлы: `routers/web/edu/admin.go`, `templates/edu/admin_panel.tmpl`

- `/edu/admin` — управление глобальными ролями пользователей (student/teacher/admin).
- Роли определяют, какие разделы видит пользователь (student vs teacher dashboards).

#### К. Маппинг ролей на организацию Forgejo

Файлы: `internal/edu/adapter.go`, `internal/edu/service_courses.go`

При записи пользователя в курс, привязанный к организации Forgejo (`OrgID > 0`), система автоматически:

1. Создаёт (или находит) команду в организации:
   - `edu-course-{id}-students` с правами `Write` (для студентов)
   - `edu-course-{id}-teachers` с правами `Admin` (для преподавателей)
2. Добавляет пользователя в соответствующую команду.
3. Команды создаются с `IncludesAllRepositories: true` — доступ ко всем репозиториям организации.

При отчислении пользователь удаляется из обеих команд. Операция идемпотентна (повторная запись не вызывает ошибок).

Интерфейс `OrgManager` в `service.go` абстрагирует эти операции. `ForgejoAdapter` реализует его через `models.NewTeam`, `models.AddTeamMember`, `models.RemoveTeamMember`.

#### Л. Каскадное удаление курса

Файл: `internal/edu/repository_courses.go`

Удаление курса выполняется в одной транзакции (`db.WithTx`) и каскадно удаляет:
1. Для каждого задания курса: `TestResult` → `Submission` → `BulkForkTask` → `SyncForkTask`
2. `Assignment` (все задания курса)
3. `ImportDraftRow` → `ImportDraft` (черновики импорта)
4. `CourseEnrollment` (записи студентов)
5. Сам `Course`

Порядок удаления — от "листьев" к "корню", чтобы не оставлять orphaned записей.

#### М. Ограничение доступа по активности курса

Файлы: `internal/edu/repository_ext.go`, `internal/edu/service_join.go`, `routers/web/edu/assignments.go`

- `GetAssignmentsForUser` фильтрует по `edu_courses.end_unix = 0 OR end_unix > now()` — студент не видит задания из завершённых курсов.
- `JoinAssignment` проверяет `course.IsActive()` — нельзя взять задание из завершённого курса.
- `AssignmentDetail` проверяет enrollment — студент не может просматривать задания из курсов, в которых не записан.

### 2.6 Полная карта роутов

```
/edu
├── /dashboard                          → Redirect по роли (teacher→assignments, student→assignments)
│
├── /student
│   ├── /assignments                    → Список заданий (только по enrolled курсам)
│   ├── /assignments/{id}               → Детали задания + CI результат + оценка
│   └── /assignments/{id}/join          → POST: взять задание (fork + submission)
│
├── /teacher                                [reqEduTeacher middleware — требует роль teacher/admin]
│   ├── /assignments                    → Список заданий преподавателя
│   ├── /assignments/new                → GET/POST: создать задание
│   ├── /assignments/{id}/edit          → GET/POST: редактировать задание
│   ├── /assignments/{id}/delete        → POST: удалить задание
│   ├── /assignments/{id}/submissions   → Таблица работ студентов
│   ├── /assignments/{id}/bulk-fork        → POST: массовый форк
│   ├── /assignments/{id}/bulk-fork-status → GET: статус/прогресс массового форка
│   ├── /assignments/{id}/sync-forks       → POST: синхронизация форков
│   ├── /assignments/{id}/sync-fork-status → GET: статус/прогресс синхронизации
│   ├── /assignments/{id}/submissions/{subID}             → Детали submission
│   ├── /assignments/{id}/submissions/{subID}/grade       → POST: выставить оценку
│   ├── /assignments/{id}/submissions/{subID}/reset-grade → POST: сброс на авто-оценку CI
│   ├── /dashboard                      → Redirect на /teacher/assignments
│   │
│   └── /courses
│       ├── /                           → Список курсов
│       ├── /new                        → GET/POST: создать курс
│       ├── /{id}                       → Детали курса (участники)
│       ├── /{id}/edit                  → GET/POST: редактировать курс
│       ├── /{id}/delete                → POST: удалить курс
│       ├── /{id}/enroll                → POST: записать студента
│       ├── /{id}/unenroll              → POST: отчислить студента
│       ├── /{id}/import                → GET/POST: загрузка CSV
│       ├── /{id}/import/{draftID}/preview    → GET: предпросмотр импорта
│       ├── /{id}/import/{draftID}/update-row → POST: правка строки
│       ├── /{id}/import/{draftID}/execute    → POST: выполнить импорт
│       └── /{id}/import/{draftID}/delete     → POST: удалить черновик
│
└── /admin                                  [reqEduAdmin middleware — требует Forgejo site admin]
    ├── /                               → Панель управления ролями
    └── /roles                          → POST: обновить роль пользователя
```

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

Edu-модуль использует **два уровня** авторизации:

#### Middleware (роутерный уровень)

Файл `routers/web/edu/middleware.go` содержит два middleware:

- **`reqEduTeacher`** — проверяет, что у текущего пользователя edu-роль `teacher` или `admin` (через таблицу `edu_user_role`). Применяется ко всей группе `/edu/teacher/*`. Если роль отсутствует — возвращает 403.
- **`reqEduAdmin`** — проверяет, что пользователь является Forgejo site admin (`ctx.Doer.IsAdmin`). Применяется к `/edu/admin/*`.

#### Проверка владения курсом (хендлерный уровень)

Мутирующие операции над курсами (edit, delete, enroll, unenroll, import) дополнительно проверяют `course.CreatorID == ctx.Doer.ID`. Это гарантирует, что один преподаватель не может редактировать курсы другого. Если проверка не проходит — возвращается 403.

#### Проверка прав доступа к репозиторию (Forgejo core)

Для операций с репозиториями используется Forgejo core:

```go
perm, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
if !perm.IsAdmin() && !perm.CanWrite(unit_model.TypeCode) {
    ctx.Error(http.StatusForbidden, "Only instructors can view this page")
    return
}
```

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
