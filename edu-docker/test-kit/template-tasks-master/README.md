# tasks-master (template для одного курса)

Это шаблон единственного на курс репо `tasks-master`. Он содержит:

- `tasks/<task_name>/` — папки заданий (по одной на каждое задание).
- `.forgejo/workflows/grade.yml` — общий CI-workflow для всех заданий: запускается на push в любую ветку `submits/**`.

## Как преподаватель добавляет новое задание

1. Создаёт ветку `submits/<task_name>` от `main` (можно с пустым коммитом).
2. На `main` добавляет папку `tasks/<task_name>/` со скелетом + тестами.
3. На той же ветке `submits/<task_name>` коммитит и пушит тот же скелет — это будет starting point для студентов.
4. В edu-UI создаёт Assignment с `task_name=<task_name>` и `allowed_files_glob=tasks/<task_name>/<file>`.
5. Жмёт **Distribute** — ветка раздаётся в студенческие форки.

## Convention

- Студент имеет право трогать только файлы внутри `tasks/<task_name>/`. CI-сторона делает дешёвую проверку по этой конвенции; авторитетный `AllowedFilesGlob` проверяется на сервере (notifier).
- Финальный шаг workflow — `echo "::edu-grade::XX"` — задаёт оценку 0–100. На сервере она считается preliminary, пока TA не нажал Approve в edu-UI.
