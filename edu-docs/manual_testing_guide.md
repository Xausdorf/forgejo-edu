# Руководство по ручному тестированию (Manual Testing Guide)

Этот документ описывает шаги для проверки всех функций образовательного расширения Forgejo.

## Подготовка

1. Соберите и запустите Forgejo:

   **Вариант A — Docker (Windows/Mac/Linux):**
   ```bash
   cd forgejo-edu
   docker compose -f edu-docker/docker-compose.yml up forgejo --build
   ```

   **Вариант B — Нативно (только Linux):**
   ```bash
   cd forgejo-edu
   make build
   go run main.go web
   ```

   Сервер запустится на `http://localhost:3000`. Первый зарегистрированный пользователь станет администратором.

2. Убедитесь, что у вас есть пользователи с ролями. Зайдите на `/edu/admin` под администратором и назначьте роли:
   - `user1` → `teacher`
   - `user2` → `student`
   - `user3` → `student` (для проверки, что не видит чужие курсы)

3. У преподавателя должен быть `tasks-master` репозиторий в org-е с примером задания (`tasks/sample/`) и `.forgejo/workflows/grade.yml`. Используйте `edu-docker/test-kit/template-tasks-master/` как заготовку.

---

## Сценарий 1: Курсы

### 1.1 Создание курса

1. Войдите как `user1` (teacher).
2. Перейдите на `/edu/teacher/courses`.
3. Нажмите "New Course".
4. Заполните: Name = "Программирование 101", Description = "Основы", Start/End даты.
5. Нажмите "Create".

**Ожидаемый результат**: Вы перенаправлены на список курсов, курс "Программирование 101" отображается.

### 1.2 Детали курса

1. Нажмите на "Программирование 101" в списке.

**Ожидаемый результат**: Страница с названием, описанием, датами, пустой таблицей участников.

### 1.3 Редактирование курса

1. На странице курса нажмите "Edit".
2. Измените описание на "Основы программирования на Go".
3. Нажмите "Save".

**Ожидаемый результат**: Курс обновлён, описание изменилось.

### 1.4 Удаление курса

1. Создайте ещё один курс "Тестовый курс".
2. На странице этого курса нажмите "Delete" и подтвердите.

**Ожидаемый результат**: Курс удалён, вы перенаправлены на список.

---

## Сценарий 2: Запись студентов (Enrollment)

### 2.1 Ручная запись

1. Войдите как `user1`, откройте курс "Программирование 101".
2. В форме "Add Participant" введите `user2`, выберите роль `student`, нажмите "Add".

**Ожидаемый результат**: `user2` появился в таблице участников с ролью `student`.

### 2.2 Удаление студента

1. В таблице участников нажмите "Remove" напротив `user2`.

**Ожидаемый результат**: `user2` исчез из таблицы.

### 2.3 CSV-импорт

1. Запишите `user2` обратно (ручная запись, чтобы не потерять).
2. На странице курса нажмите "Import from CSV".
3. Подготовьте CSV-файл `students.csv`:
   ```
   Иванов Иван;ivanov@test.com
   Петрова Анна;petrova@test.com
   ```
4. Выберите файл и укажите маппинг колонок: Full Name = 0, Email = 1, Has Header = No.
5. Нажмите "Upload".

**Ожидаемый результат**: Страница предпросмотра показывает таблицу:
| Full Name | Username | Email | Status |
|-----------|----------|-------|--------|
| Иванов Иван | ivanov-i | ivanov@test.com | pending |
| Петрова Анна | petrova-a | petrova@test.com | pending |

6. Можно отредактировать username/email, нажав "Update" у строки.
7. Нажмите "Execute Import".

**Ожидаемый результат**: Пользователи созданы, таблица с логинами/паролями для раздачи. Новые пользователи записаны в курс.

---

## Сценарий 3: Задания

### 3.1 Создание задания

1. Войдите как `user1`.
2. Перейдите на `/edu/teacher/assignments/new`.
3. Выберите курс "Программирование 101", заполните `task_name=multiplication`, `allowed_files_glob=tasks/multiplication/multiplication.cpp`.
4. Заполните: Title = "ДЗ 1", Description = "Решите задачи", Deadline = будущая дата.
5. Нажмите "Create".

**Ожидаемый результат**: Задание создано, вы видите его в списке `/edu/teacher/assignments`.

### 3.2 Редактирование задания

1. Нажмите "Edit" у задания.
2. Измените описание, нажмите "Save".

**Ожидаемый результат**: Описание обновлено.

### 3.3 Просмотр списка (студент)

1. Войдите как `user2` (студент, записанный в курс).
2. Перейдите на `/edu/student/assignments`.

**Ожидаемый результат**: Задание "ДЗ 1" отображается в списке.

3. Войдите как `user3` (не записан в курс).
4. Перейдите на `/edu/student/assignments`.

**Ожидаемый результат**: Список пуст — `user3` не видит чужие задания.

---

## Сценарий 4: Init forks (инициализация студенческих форков)

### 4.1 Успешный запуск

1. Создать курс с `OrgID` и `TasksMasterRepoID`.
2. Зачислить 3 студентов (через ручную форму или CSV).
3. На странице курса нажать **Init forks**.
4. Дождаться `Status=done` на странице `/init-forks-status`.

Ожидание: для каждого студента в org-е появилось репо `<username>-tasks`. На `main` форка стоит branch protection (нельзя push). Студент — Write-collaborator на свой fork. `Enrollment.StudentForkRepoID` заполнен.

### 4.2 Повторный init-forks

1. Зачислить ещё одного студента.
2. Снова нажать **Init forks**.

Ожидание: пропускает уже инициализированных, форкает только новенького.

### 4.3 Запуск без `TasksMasterRepoID`

1. Создать курс без `TasksMasterRepoID`.
2. Открыть страницу курса, нажать **Init forks**.

Ожидание: flash error «Course has no tasks-master repo configured» / соответствующий ключ локали; никаких форков не создано.

## Сценарий 5: Distribute (раздача задания)

### 5.1 Успешная раздача

1. Курс с инициализированными форками (см. 4.1).
2. В `tasks-master` запушить ветку `submits/multiplication` (можно пустой коммит на base).
3. Создать задание (`task_name=multiplication`, `allowed_files_glob=tasks/multiplication/multiplication.cpp`).
4. Нажать **Distribute**.

Ожидание: на `/distribute-status` `Status=done`, `Pushed=N`, `Failed=0`. У каждого студента в его fork-е появилась ветка `submits/multiplication`. В `/edu/student/assignments` у каждого студента запись со статусом `pending`.

### 5.2 Отсутствие ветки `submits/<task>` в `tasks-master`

1. Создать задание с `task_name=nonexistent` (ветки в `tasks-master` нет).

Ожидание: при сабмите формы — flash error «Branch submits/nonexistent not found in tasks-master»; задание не создано.

### 5.3 Distribute в неактивном курсе

1. Установить `EndUnix` курса в прошлое.
2. Нажать **Distribute**.

Ожидание: flash error / 403; никаких пушей.

## Сценарий 6: CI/CD и notifier

### 6.1 Успешный submit

1. Студент клонирует свой fork, чекаутит `submits/multiplication`, пишет код, пушит.

Ожидание: 
- CI workflow `grade.yml` запустился, прошёл, вывел `::edu-grade::100`.
- `Submission.Status = done`.
- В fork-е создан PR `submits/multiplication` → `main` с тайтлом `[<group>] Submit: multiplication` (или просто `Submit: multiplication` если `GroupName==""`).
- В edu-UI у submission-а появилась оценка 100.
- Создана запись `TestResult` с `CommitSHA`.

### 6.2 Constraint check (попытка изменить запрещённый файл)

1. Студент в ветке `submits/multiplication` правит файл вне `tasks/multiplication/`.
2. Пушит.

Ожидание:
- Server-side notifier ставит `Status = failed`.
- В PR (если уже создан) — системный комментарий «Forbidden files changed: ...».
- CI тоже падает на cheap convention check.

### 6.3 Auto-grade с дробным результатом

1. Студент пишет частично рабочий код (1 из 2 тестов проходят).
2. CI выводит `echo "::edu-grade::50"`.

Ожидание: `Submission.Grade = 50`, `Status = done`.

## Сценарий 7: Submission review (Approve / Merge)

### 7.1 Approve

1. У сабмита `Status=done`.
2. TA открывает `/edu/teacher/assignments/{id}/submissions/{subID}`.
3. Видит diff и комментарии PR на странице.
4. Оставляет inline-комментарий.
5. Жмёт **Approve** с grade=85, comment="LGTM, but consider X".

Ожидание: `Status=approved`, `Grade=85`, `ManualGrade=true`, `Comment="LGTM, but consider X"`. Кнопка **Merge** активна, кнопка **Approve** скрыта.

### 7.2 Merge

1. Из 7.1 нажать **Merge**.

Ожидание: ветка `submits/multiplication` смержена в `main` форка от имени `eduadmin`. `Status=merged`. PR закрыт.

### 7.3 ManualGrade перекрывает CI

1. После 7.1 студент пушит снова, CI выставляет 100.

Ожидание: `Submission.Grade` остаётся 85 (не 100), потому что `ManualGrade=true`.

### 7.4 Reset Approval

1. Из 7.1 нажать **Reset Approval**.

Ожидание: `Status=done`, `ManualGrade=false`. Если придёт новый CI с auto-grade — он будет применён.

## Сценарий 8: Course sync

### 8.1 Sync без конфликтов

1. Зайти в `/edu/teacher/courses/{id}/sync`.
2. Нажать **Запустить синхронизацию**.

Ожидание: для каждого студенческого fork-а пушнулась ветка `course-sync`, открыт PR `course-sync` → `main`, auto-merge сработал. В таблице все строки `merged`.

### 8.2 Sync с конфликтом

1. В `tasks-master` `main` поправить файл, который один из студентов трогал в своём `main`.
2. Запустить sync.

Ожидание: у того студента строка `conflict`, PR в его fork-е остался открытым. У остальных — `merged`. Кнопка **Merge все без конфликтов** работает только на зелёных.

### 8.3 Фильтр по группе

1. В таблице sync выбрать `?group=se241`.

Ожидание: видно только PR-ы студентов из группы se241.

---

## Сценарий 9: Валидации формы оценки

### 9.1 Валидация диапазона

1. Войдите как `user1`, откройте Detail submission.
2. Попробуйте ввести Grade = 150 или Grade = -1.

**Ожидаемый результат**: Flash-ошибка "Grade must be between 0 and 100".

---

## Сценарий 10: Dashboard

1. Войдите как `user1` (teacher), перейдите на `/edu/dashboard`.
   **Ожидаемый результат**: Redirect на `/edu/teacher/assignments`.

2. Войдите как `user2` (student), перейдите на `/edu/dashboard`.
   **Ожидаемый результат**: Redirect на `/edu/student/assignments`.

---

## Сценарий 11: Навигация

1. Войдите любым пользователем.
2. Проверьте наличие ссылки "Education" в навигационной панели.

**Ожидаемый результат**: Ссылка видна и ведёт на `/edu/dashboard`.

---

## Сценарий 12: Админ-панель

1. Войдите как администратор Forgejo.
2. Перейдите на `/edu/admin`.
3. Найдите пользователя, выберите роль (`student` / `teacher` / `admin`), нажмите "Update".

**Ожидаемый результат**: Роль обновлена. Пользователь теперь видит соответствующий dashboard.

---

## Сценарий 13: Авторизация и защита маршрутов

### 13.1 Студент не может зайти на маршруты преподавателя

1. Войдите как `user2` (student).
2. Перейдите вручную на `/edu/teacher/assignments`.

**Ожидаемый результат**: HTTP 403 Forbidden. Студент не видит страницу преподавателя.

3. Попробуйте также `/edu/teacher/courses`, `/edu/teacher/courses/new`.

**Ожидаемый результат**: Все маршруты `/edu/teacher/*` возвращают 403 для студента.

### 13.2 Студент не может зайти на админ-панель

1. Войдите как `user2` (student).
2. Перейдите на `/edu/admin`.

**Ожидаемый результат**: HTTP 403 Forbidden.

### 13.3 Преподаватель не может редактировать чужой курс

1. Создайте второго преподавателя: войдите как администратор, на `/edu/admin` назначьте `user4` роль `teacher`.
2. Войдите как `user1` (teacher), создайте курс "Курс user1".
3. Войдите как `user4` (teacher), перейдите вручную на `/edu/teacher/courses/{id}/edit` (подставив ID курса user1).

**Ожидаемый результат**: HTTP 403 Forbidden. Преподаватель не может редактировать чужой курс.

4. Аналогично проверьте: `/edu/teacher/courses/{id}/delete` (POST), `/edu/teacher/courses/{id}/enroll` (POST), `/edu/teacher/courses/{id}/import`.

**Ожидаемый результат**: Все мутирующие операции возвращают 403 для не-владельца курса.

---

## Сценарий 14: Защита от дублирования записей (Enrollment)

### 14.1 Повторная запись студента в курс

1. Войдите как `user1` (teacher), откройте курс.
2. Запишите `user2` в курс (если ещё не записан).
3. Попробуйте записать `user2` повторно через форму "Add Participant".

**Ожидаемый результат**: Ошибка. Студент не дублируется в таблице участников. UNIQUE constraint на `(CourseID, UserID)` предотвращает повторную запись.

---

## Автоматические тесты

```bash
# Unit-тесты edu-модуля
cd forgejo-edu
go test ./internal/edu/...

# Интеграционные тесты (SQLite in-memory)
make test-sqlite#TestEdu

# Конкретный тест
make test-sqlite#TestEduCourseCreate

# E2E тесты (Playwright, нужен npx playwright install)
make test-e2e-sqlite
```

## CI/CD Runner (Forgejo Actions)

Для тестирования CI/CD (раздел 6) необходим Forgejo Actions runner. Он включён в `docker-compose.yml`:

```bash
# Запуск Forgejo + runner
docker compose -f edu-docker/docker-compose.yml up forgejo forgejo-runner --build
```

Runner автоматически регистрируется при первом запуске (требует пользователя `eduadmin` с паролем `Password123!`).
Если авто-регистрация не сработала — зарегистрируйте вручную:
1. Откройте `http://localhost:3000/-/admin/runners`
2. Скопируйте Registration Token
3. `docker exec -it <runner-container> forgejo-runner register --instance http://forgejo:3000 --token <TOKEN> --name edu-runner --labels ubuntu-latest:docker://node:20-bookworm --no-interactive`

---

## Расширенный тест-кит

Полный набор тестовых данных и скриптов находится в каталоге `edu-docker/test-kit/`:

- `edu-docker/test-kit/TESTING_PLAN.md` — Полный план (~133 тест-кейсов, 15 разделов)
- `edu-docker/test-kit/setup.sh` — Скрипт создания тестового окружения (пользователи, орг, репозитории, роли)
- `edu-docker/test-kit/csv/` — 8 CSV-файлов для тестирования импорта (UTF-8, Win-1251, BOM, XSS, пустой, невалидный)
- `edu-docker/test-kit/template-tasks-master/` — Шаблон `tasks-master` репозитория с примером задания (`tasks/sample/`), CI/CD workflow (`grade.yml`) и инструкцией для преподавателя

---

## Ограничения

- Автоматическое обновление UI в реальном времени не реализовано — нужна перезагрузка страницы.
- Нет email-нотификаций.
