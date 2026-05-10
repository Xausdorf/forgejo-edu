# Test Kit — Ручное тестирование Forgejo Edu

Набор скриптов, данных и инструкций для полного ручного тестирования образовательного расширения.

## Структура

```
test-kit/
  TESTING_PLAN.md              # Полный план тестирования (~133 тест-кейса)
  setup.sh                     # Скрипт создания тестового окружения
  README.md                    # Этот файл
  csv/                         # Тестовые CSV-файлы для импорта
    students_basic.csv         # UTF-8, точка с запятой, без заголовка
    students_with_header.csv   # UTF-8, запятая, с заголовком
    students_with_group.csv    # 3 колонки (ФИО, email, группа)
    students_bom.csv           # UTF-8 с BOM
    students_win1251.csv       # Windows-1251
    students_xss.csv           # XSS-атака в именах (тест безопасности)
    empty.csv                  # Пустой файл
    invalid.csv                # Невалидные данные
    README.md                  # Описание CSV-файлов
  template-tasks-master/       # Шаблон единственного course repo (GitLab-модель)
    README.md                  # Описание конвенций tasks-master
    .forgejo/workflows/grade.yml  # CI workflow (запуск на submits/**)
    tasks/sample/              # Пример задания
      sample.py                # Заглушка (студент реализует)
      test_sample.py           # Тесты pytest
      README.md                # Инструкция для студента
```

## Быстрый старт

### 1. Запуск Forgejo

```bash
cd forgejo-edu
docker compose -f edu-docker/docker-compose.yml down -v   # сброс (опционально)
docker compose -f edu-docker/docker-compose.yml up forgejo --build
```

### 2. Регистрация первого пользователя

Откройте http://localhost:3000, зарегистрируйте `eduadmin` / `Password123!` / `eduadmin@localhost.local`.
Первый зарегистрированный пользователь автоматически становится site admin.

### 3. Запуск setup-скрипта

```bash
bash test-kit/setup.sh
```

Скрипт создаст:
- 6 пользователей (teacher1, teacher2, student1-3, norole)
- Организацию `test-org`
- Репозиторий `test-org/tasks-master` (с CI-workflow и задачей-примером `tasks/sample/`)
- Назначит edu-роли через веб-сессию

### 4. Прохождение тестов

Откройте `TESTING_PLAN.md` и проходите сценарии по порядку, отмечая статус каждого кейса.
