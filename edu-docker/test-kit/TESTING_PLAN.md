# Полный план ручного тестирования образовательного расширения Forgejo

## Тестовое окружение

### Запуск

```bash
cd forgejo-edu
docker compose -f edu-docker/docker-compose.yml down -v   # сброс (при необходимости)
docker compose -f edu-docker/docker-compose.yml up forgejo --build
```

Сервер: `http://localhost:3000`

### Тестовые аккаунты

Создаются скриптом `setup.sh`. Пароль для всех: `Password123!`

| Логин       | Роль edu   | Site Admin | Назначение                              |
|-------------|------------|------------|-----------------------------------------|
| `eduadmin`  | admin      | да         | Администратор Forgejo + edu             |
| `teacher1`  | teacher    | нет        | Основной преподаватель, владелец курсов |
| `teacher2`  | teacher    | нет        | Второй преподаватель (тест изоляции)    |
| `student1`  | student    | нет        | Записан в курс                          |
| `student2`  | student    | нет        | Записан в курс                          |
| `student3`  | student    | нет        | НЕ записан — проверка изоляции          |
| `norole`    | (нет)      | нет        | Пользователь без edu-роли               |

### Организация

| Название       | Владелец   | Назначение                      |
|----------------|------------|---------------------------------|
| `test-org`     | `teacher1` | Орг привязанная к курсу         |

### Репозитории

- `test-org/tasks-master` — единственный course repo с задачей-примером (`tasks/sample/`) и `.forgejo/workflows/grade.yml`. Содержит ветки `main` и `submits/sample`.
- `<student>-tasks` (по одному на каждого зачисленного, в том же `test-org`) — создаётся на этапе Init forks. До init-forks не существует.

---

## Чек-лист тестирования

### Условные обозначения

- **[P]** — Passed
- **[F]** — Failed
- **[S]** — Skipped
- **[B]** — Blocked

---

## 1. Админ-панель и управление ролями

### 1.1 Доступ к админ-панели

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Войти как `eduadmin`, перейти на `/edu/admin` | Страница загружается, видна форма управления ролями | [P] |
| 2 | Войти как `teacher1`, перейти на `/edu/admin` | HTTP 403 Forbidden | [P] |
| 3 | Войти как `student1`, перейти на `/edu/admin` | HTTP 403 Forbidden | [P] |
| 4 | Войти как `norole`, перейти на `/edu/admin` | HTTP 403 Forbidden | [P] |
| 5 | Не авторизован, перейти на `/edu/admin` | Redirect на страницу входа | [P] |

### 1.2 Назначение ролей

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | На `/edu/admin` найти `teacher1`, выбрать `teacher`, нажать "Update" | Роль `teacher` сохранена | [P] |
| 2 | Найти `student1`, назначить `student` | Роль `student` сохранена | [P] |
| 3 | Найти `norole`, назначить `student` | Роль `student` сохранена | [P] |
| 4 | Проверить: `norole` теперь видит `/edu/student/assignments` | Страница загружается (200 OK) | [P] |
| 5 | Снять роль у `norole` (назначить пустую / удалить) | Пользователь снова без роли | [P] "Нет (убрать роль)" опция добавлена, "Role removed" flash работает |

---

## 2. Dashboard и навигация

### 2.1 Dashboard redirect

| # | Вход как   | URL               | Ожидаемый redirect                 | Статус |
|---|------------|--------------------|------------------------------------|--------|
| 1 | `teacher1` | `/edu/dashboard`   | `/edu/teacher/assignments`         | [P] |
| 2 | `student1` | `/edu/dashboard`   | `/edu/student/assignments`         | [P] |
| 3 | `eduadmin` | `/edu/dashboard`   | Redirect по роли (admin → teacher) | [P] |
| 4 | `norole`   | `/edu/dashboard`   | 403 или redirect на главную        | [P] redirect на /edu/student/assignments (student routes доступны всем залогиненным) |

### 2.2 Навигационная панель

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Войти любым пользователем | Ссылка "Education" видна в навбаре | [P] |
| 2 | Нажать "Education" | Переход на `/edu/dashboard` | [P] |

---

## 3. Курсы (CRUD)

### 3.1 Создание курса

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Войти как `teacher1`, `/edu/teacher/courses/new` | Форма с полями: Name, Description, Start Date, End Date, Organization (dropdown) | [P] |
| 2 | Заполнить: Name="Программирование Go", Description="Основы", Start=сегодня, End=через месяц, Org=`test-org`. Нажать Create | Redirect на список курсов, курс виден | [P] |
| 3 | Создать курс без организации: Name="Математика", без Org | Курс создан, OrgID=0 | [P] |
| 4 | Создать курс с пустым именем | Ошибка валидации, курс не создан | [P] |
| 5 | Создать курс с End Date < Start Date | Поведение: курс создан или ошибка (зафиксировать) | [S] |

### 3.2 Просмотр курса

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Нажать на "Программирование Go" в списке | Страница детали: название, описание, даты, пустая таблица участников | [P] |
| 2 | Проверить наличие кнопок: Edit, Delete, Import from CSV, Add Participant | Все кнопки/формы присутствуют | [P] |

### 3.3 Редактирование курса

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Нажать Edit, изменить Description на "Основы Go и тестирования", Save | Описание обновлено | [P] |
| 2 | Изменить End Date на прошлую дату, Save | Курс сохранен (станет "expired") | [S] |
| 3 | Вернуть End Date в будущее | Курс снова активен | [S] |

### 3.4 Удаление курса (каскад)

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Создать курс "Удалить меня" | Курс создан | [P] |
| 2 | Записать `student1` в курс | Enrollment создан | [P] |
| 3 | Создать задание в этом курсе | Assignment создан | [P] |
| 4 | Нажать Delete на курсе, подтвердить | Курс удалён, redirect на список | [P] |
| 5 | Проверить: задание удалено (не видно в списке заданий) | Каскадное удаление сработало | [P] |
| 6 | Проверить: enrollment удалён (student1 не видит курс) | Каскадное удаление enrollments | [P] |

### 3.5 Изоляция курсов между преподавателями

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Войти как `teacher2`, `/edu/teacher/courses` | Список курсов teacher2 (пустой, если не создавал) | [P] |
| 2 | Перейти вручную на `/edu/teacher/courses/{id}/edit` курса teacher1 | HTTP 403 Forbidden | [P] |
| 3 | POST `/edu/teacher/courses/{id}/delete` курса teacher1 | HTTP 403 Forbidden | [P] |
| 4 | POST `/edu/teacher/courses/{id}/enroll` курса teacher1 | HTTP 403 Forbidden | [P] |
| 5 | GET `/edu/teacher/courses/{id}/import` курса teacher1 | HTTP 403 Forbidden | [P] |

---

## 4. Запись студентов (Enrollment)

### 4.1 Ручная запись

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | На странице курса "Программирование Go" добавить `student1` с ролью `student` | student1 появился в таблице участников | [P] |
| 2 | Добавить `student2` с ролью `student` | student2 в таблице | [P] |
| 3 | Добавить `teacher2` с ролью `teacher` | teacher2 в таблице с ролью teacher | [P] |
| 4 | Попробовать добавить несуществующего пользователя `noexist` | Ошибка: пользователь не найден | [P] |
| 5 | Попробовать добавить `student1` повторно | Ошибка: уже записан (UNIQUE constraint) | [P] баг исправлен, нет 500 |

### 4.2 Удаление из курса

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Нажать "Remove" у `teacher2` | teacher2 удален из таблицы | [P] |
| 2 | Проверить: teacher2 больше не видит задания этого курса (если есть) | Изоляция работает | [S] |

### 4.3 Org team mapping (при наличии OrgID)

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Курс "Программирование Go" привязан к `test-org`. Записать `student1` | student1 добавлен в team `edu-course-{id}-students` в `test-org` | [P] |
| 2 | Проверить в Forgejo: `test-org` → Settings → Teams → `edu-course-*-students` | Team существует, student1 — участник | [P] |
| 3 | Удалить `student1` из курса | student1 удалён из team | [P] |
| 4 | Записать обратно `student1` | Повторно добавлен в team | [P] |

---

## 5. CSV-импорт студентов

### 5.1 Базовый импорт (UTF-8, точка с запятой)

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Открыть курс, нажать "Import from CSV" | Форма загрузки с полями маппинга | [P] |
| 2 | Загрузить `csv/students_basic.csv`: Full Name=0, Email=1, Has Header=No | Preview показывает таблицу | [P] |
| 3 | Проверить preview: ФИО, сгенерированные username, email, статус pending | Данные корректны, транслитерация верная | [P] |
| 4 | Нажать "Execute Import" | Пользователи созданы, показаны логины/пароли | [P] |
| 5 | Проверить: новые пользователи записаны в курс | Enrollments созданы | [P] |

### 5.2 CSV с заголовком (UTF-8, запятая)

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Загрузить `csv/students_with_header.csv`: Full Name=0, Email=1, Has Header=Yes | Preview корректный (первая строка пропущена) | [S] |
| 2 | Execute Import | Пользователи созданы | [S] |

### 5.3 CSV в кодировке Windows-1251

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Загрузить `csv/students_win1251.csv`: Full Name=0, Email=1, Has Header=No | Кириллица корректно отображается в preview | [S] |
| 2 | Execute Import | Пользователи созданы с правильными именами | [S] |

### 5.4 CSV с UTF-8 BOM

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Загрузить `csv/students_bom.csv` | BOM корректно удален, preview верный | [S] |

### 5.5 CSV с группой (3 колонки)

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Загрузить `csv/students_with_group.csv`: Full Name=0, Email=1, Group=2, Has Header=Yes | Preview показывает колонку Group | [S] |

### 5.6 Редактирование preview

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | На странице preview изменить username у первой строки | Username обновлён | [S] |
| 2 | Изменить email у второй строки | Email обновлён | [S] |
| 3 | Execute Import | Пользователи созданы с изменёнными данными | [S] |

### 5.7 Отмена импорта

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Загрузить CSV, увидеть preview | Preview отображается | [S] |
| 2 | Нажать "Cancel" / "Delete" | Черновик удалён, redirect на курс | [S] |

### 5.8 Пустой / невалидный CSV

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Загрузить `csv/empty.csv` | Ошибка или пустой preview | [S] |
| 2 | Загрузить `csv/invalid.csv` (одна колонка с числами) | Ошибка или пустые имена пропущены | [S] |

---

## 6. Задания (Assignments)

### 6.1 Создание задания

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Войти как `teacher1`, `/edu/teacher/assignments/new` | Форма: Course (dropdown), Template Repo, Title, Description, Deadline | [P] |
| 2 | Выбрать курс "Программирование Go" (с OrgID) | Страница перезагружается, repo dropdown показывает репозитории `test-org` | [P] |
| 3 | Выбрать repo=`homework-go`, Title="ДЗ 1: Hello World", Deadline=будущая дата, Create | Задание создано, видно в списке | [P] |
| 4 | Создать задание в курсе "Математика" (без Org) | Repo dropdown показывает личные репозитории teacher1 | [P] |
| 5 | Выбрать repo=`homework-simple`, Title="ДЗ Матан", Create | Задание создано | [P] |
| 6 | Попробовать создать задание без выбора курса | Ошибка валидации | [S] |
| 7 | Создать задание с дедлайном в прошлом | Задание создано (допустимо для backfill) | [S] |

### 6.2 Редактирование задания

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Открыть "ДЗ 1", нажать Edit | Форма редактирования | [S] |
| 2 | Изменить Title и Deadline, Save | Данные обновлены | [S] |

### 6.3 Удаление задания

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Создать задание "Удалить ДЗ" | Задание создано | [S] |
| 2 | Нажать Delete, подтвердить | Задание удалено | [S] |

### 6.4 Список заданий (студент)

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Войти как `student1` (записан в "Программирование Go"), `/edu/student/assignments` | "ДЗ 1: Hello World" видно в списке | [P] |
| 2 | Войти как `student3` (НЕ записан), `/edu/student/assignments` | Список пуст | [P] |
| 3 | Задание в истёкшем курсе (End Date в прошлом) | Задание НЕ видно студенту | [S] |

---

## 7. Init forks (инициализация студенческих форков)

### 7.1 Успешный init-forks
- Курс с `OrgID=test-org`, `TasksMasterRepoID=tasks-master`. Зачислены student1, student2.
- Жмём «Init forks» на странице курса.
- Ожидание: `/init-forks-status` доходит до `Status=done`. В `test-org` появились `student1-tasks` и `student2-tasks`. На `main` каждого fork-а — branch protection. Студенты — Write-collaborator на свой fork.

### 7.2 Повторный init-forks (idempotency)
- Зачислить student3 после первого init-forks.
- Жмём «Init forks» снова.
- Ожидание: `student1-tasks` и `student2-tasks` не пересоздаются; `student3-tasks` создаётся.

### 7.3 Init без `TasksMasterRepoID`
- Создать курс без `TasksMasterRepoID`.
- Жмём «Init forks».
- Ожидание: flash error; ничего не создано.

### 7.4 Init в неактивном курсе
- Установить `EndUnix` в прошлое.
- Жмём «Init forks».
- Ожидание: flash error / 403.

## 8. Distribute (раздача задания)

### 8.1 Успешная раздача
- Курс из 7.1 (форки уже инициализированы). Создать assignment `task_name=sample`, `allowed_files_glob=tasks/sample/sample.py`.
- Жмём «Distribute».
- Ожидание: `/distribute-status` дошёл до `done`, `Pushed=2`, `Failed=0`. У student1-tasks и student2-tasks появилась ветка `submits/sample`. В `/edu/student/assignments` обоих студентов запись `pending`.

### 8.2 Distribute без существующей ветки в tasks-master
- Создать assignment `task_name=missing` (ветки `submits/missing` нет в tasks-master).
- Ожидание: форма создания не сохранится — flash error «Branch submits/missing not found».

### 8.3 Повторный distribute
- Жмём «Distribute» ещё раз.
- Ожидание: ветки уже есть, push идёт без эффекта; submission-ы остаются `pending` (UPSERT, статус не сбрасывается).

### 8.4 Distribute задания с группой
- Зачислить student2 с `GroupName=se241`.
- Жмём «Distribute».
- Ожидание: на странице submission-ов фильтр `?group=se241` показывает только student2.

## 9. Submission review (Approve / Merge)

### 9.1 Успешный submit + auto-PR
- Student1 клонирует `student1-tasks`, чекаутит `submits/sample`, реализует `add` корректно, пушит.
- Ожидание (после CI): `Status=done`, `Grade=100`, в `student1-tasks` создан PR `submits/sample` → `main`. Title `Submit: sample` (без префикса, если у student1 нет группы).

### 9.2 Constraint check
- Student1 правит файл вне `tasks/sample/` (например, `.forgejo/workflows/grade.yml`).
- Ожидание: server-side `Status=failed`, в PR — системный комментарий «Forbidden files: ...». CI тоже падает.

### 9.3 Approve и Merge
- TA открывает submission-страницу. Видит diff и комментарии PR.
- TA пишет inline-комментарий через форму на той же странице.
- TA жмёт Approve grade=85 comment="LGTM".
- Ожидание: `Status=approved`, `Grade=85`, `ManualGrade=true`. Кнопка Merge активна.
- TA жмёт Merge.
- Ожидание: ветка смержена в `main` от имени `eduadmin`. `Status=merged`. PR закрыт.

### 9.4 ManualGrade перекрывает CI
- После 9.3 student1 пушит ещё один коммит. CI выставляет 100.
- Ожидание: `Submission.Grade` остаётся 85.

### 9.5 Reset Approval
- TA жмёт Reset Approval.
- Ожидание: `Status=done`, `ManualGrade=false`. Следующий CI снова обновит grade.

### 9.6 PR title с группой
- Student2 (с `GroupName=se241`) делает push.
- Ожидание: notifier создаёт PR с заголовком `[se241] Submit: sample`.

## 10. Course sync

### 10.1 Sync без конфликтов
- Преподаватель в `tasks-master` `main` правит README (не трогая `tasks/sample/`).
- Идём на `/edu/teacher/courses/{id}/sync`, жмём «Запустить».
- Ожидание: для каждого студенческого fork-а пушнулась ветка `course-sync`, открыт PR, auto-merge сработал. В таблице все строки `merged`.

### 10.2 Sync с конфликтом
- В `tasks-master` поправить README. У student1 в `main` тоже изменён README (например, через прямой commit от owner-а раньше; в реальном сценарии — конфликт через какой-то общий файл).
- Запустить sync.
- Ожидание: у student1 строка `conflict`, PR в его fork-е остался открытым. У остальных `merged`.

### 10.3 Merge all without conflicts
- Из 10.2 жмём «Merge все без конфликтов».
- Ожидание: все `pending`/без-конфликта PR-ы смержились; конфликтные остались.

### 10.4 Ручной merge одного PR
- Открыть PR student1 (через ссылку с страницы sync), вручную разрешить конфликт через Forgejo UI / git, нажать Merge на edu-странице.
- Ожидание: строка → `merged`.

### 10.5 Фильтр по группе
- Добавить student2 в `se241`. Запустить sync.
- На странице sync выбрать `?group=se241`.
- Ожидание: видно только запись student2.

---

## 11. Валидации формы оценки

### 11.1 Валидация оценки

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Grade = 150, Save | Flash: "Grade must be between 0 and 100" | [ ] |
| 2 | Grade = -1, Save | Flash: "Grade must be between 0 and 100" | [ ] |
| 3 | Grade = 0, Save | Оценка 0/100 сохранена (допустимо) | [ ] |
| 4 | Grade = 100, Save | Оценка 100/100 сохранена | [ ] |

---

## 12. Авторизация и защита маршрутов

### 12.1 Студент → маршруты преподавателя

| # | URL | Ожидаемый результат | Статус |
|---|-----|---------------------|--------|
| 1 | `/edu/teacher/assignments` | 403 | [P] |
| 2 | `/edu/teacher/courses` | 403 | [P] |
| 3 | `/edu/teacher/courses/new` | 403 | [P] |
| 4 | `/edu/teacher/assignments/new` | 403 | [P] |
| 5 | `/edu/teacher/assignments/{id}/submissions` | 403 | [P] |
| 6 | POST `/edu/teacher/assignments/{id}/bulk-fork` | 403 | [S] |
| 7 | POST `/edu/teacher/assignments/{id}/sync-forks` | 403 | [S] |

### 12.2 Преподаватель → маршруты админа

| # | URL | Ожидаемый результат | Статус |
|---|-----|---------------------|--------|
| 1 | `/edu/admin` | 403 | [P] |
| 2 | POST `/edu/admin/roles` | 403 | [P] |

### 12.3 Пользователь без роли

| # | URL | Ожидаемый результат | Статус |
|---|-----|---------------------|--------|
| 1 | `/edu/teacher/assignments` | 403 | [P] |
| 2 | `/edu/student/assignments` | 403 | [P] student routes доступны всем залогиненным |
| 3 | `/edu/admin` | 403 | [P] |
| 4 | `/edu/dashboard` | 403 или redirect | [P] |

### 12.4 Неавторизованный пользователь

| # | URL | Ожидаемый результат | Статус |
|---|-----|---------------------|--------|
| 1 | `/edu/teacher/assignments` | Redirect на login | [P] |
| 2 | `/edu/student/assignments` | Redirect на login | [P] |
| 3 | `/edu/admin` | Redirect на login | [P] |

### 12.5 Изоляция между преподавателями (мутации)

| # | Действие teacher2 над курсом teacher1 | Ожидаемый результат | Статус |
|---|----------------------------------------|---------------------|--------|
| 1 | GET `/edu/teacher/courses/{id}/edit` | 403 | [P] |
| 2 | POST `/edu/teacher/courses/{id}/edit` | 403 | [P] |
| 3 | POST `/edu/teacher/courses/{id}/delete` | 403 | [P] |
| 4 | POST `/edu/teacher/courses/{id}/enroll` | 403 | [P] |
| 5 | POST `/edu/teacher/courses/{id}/unenroll` | 403 | [S] |
| 6 | GET `/edu/teacher/courses/{id}/import` | 403 | [P] |
| 7 | POST `/edu/teacher/courses/{id}/import` | 403 | [S] |

---

## 13. Защита от дублирования

### 13.1 Повторная запись в курс

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | student1 уже записан. Попробовать enroll повторно | Ошибка, дубликат не создан | [P] баг исправлен |

---

## 14. Edge Cases

### 14.1 Истёкший курс

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Установить End Date курса в прошлое | Курс expired | [S] |
| 2 | Student: `/edu/student/assignments` | Задания истёкшего курса НЕ видны | [S] |
| 3 | Student: прямой URL на задание | Join невозможен | [S] |
| 4 | Teacher: задания видны, можно ставить оценки | Функционал преподавателя работает | [S] |

### 14.2 Курс без End Date (бессрочный)

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Создать курс с End Date = пустой (0) | Курс создан | [S] |
| 2 | Student видит задания этого курса | Задания видны (IsActive = true) | [S] |

### 14.3 XSS-защита

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Создать курс с Name = `<script>alert('xss')</script>` | Тег экранирован, скрипт не выполняется | [P] |
| 2 | Создать задание с Description = `<img onerror=alert(1) src=x>` | HTML экранирован | [S] |
| 3 | CSV импорт с FullName = `<b>bold</b>` | Тег экранирован в preview | [S] |

### 14.4 Длинные значения

| # | Шаг | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1 | Курс с Name = 500 символов | Создан или ошибка валидации | [S] |
| 2 | Оценка с Comment = 10000 символов | Сохранён или обрезан | [S] |

---

## 15. Полный сквозной сценарий (E2E)

### Шаги
1. Site admin создаёт teacher1 → teacher, student1 / student2 → student через `/edu/admin`.
2. Teacher1 создаёт курс «Programming-CXX» с `OrgID=test-org`, `TasksMasterRepoID=tasks-master`.
3. Через CSV-импорт (`students_with_group.csv`) зачисляет student1 (group=se241) и student2 (group=se242).
4. Жмёт «Init forks» — в `test-org` появляются `student1-tasks` и `student2-tasks` (с branch protection и collaborator-доступом).
5. Создаёт assignment `task_name=sample`, `allowed_files_glob=tasks/sample/sample.py`, deadline = +7 дней.
6. Жмёт «Distribute» — в обоих fork-ах появляется ветка `submits/sample`; в `/edu/student/assignments` оба видят `pending`.
7. Student1 клонирует, чекаутит `submits/sample`, реализует `add` корректно, пушит.
8. CI workflow `grade.yml` запускается, проходит, пишет `::edu-grade::100`.
9. Notifier создаёт PR с заголовком `[se241] Submit: sample`. `Status=done`, `Grade=100`.
10. Teacher1 (как TA) открывает submission, оставляет комментарий, жмёт Approve grade=95, потом Merge.
11. `Status=merged`. Student1 видит финальную оценку 95.

### Ожидаемый результат
Курс прошёл цикл от создания до зачётной оценки одного студента; все edu-таблицы содержат правильные записи; форки student1 и student2 живут в org-е; ветка `submits/sample` смержена в `main` форка student1.

---

## Итого: количество тест-кейсов

15 сценариев, ~70 чек-кейсов после рефакторинга под GitLab-модель.
