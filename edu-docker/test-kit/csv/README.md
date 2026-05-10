# Тестовые CSV-файлы

| Файл | Описание | Кодировка | Разделитель | Заголовок |
|------|----------|-----------|-------------|-----------|
| `students_basic.csv` | 5 студентов, ФИО + email | UTF-8 | `;` | Нет |
| `students_with_header.csv` | 3 студента с заголовком | UTF-8 | `,` | Да |
| `students_with_group.csv` | 4 студента, 3 колонки (ФИО, email, группа) | UTF-8 | `;` | Да |
| `students_bom.csv` | 2 студента с UTF-8 BOM | UTF-8 + BOM | `;` | Нет |
| `students_win1251.csv` | 3 студента в Windows-1251 | Windows-1251 | `;` | Нет |
| `students_xss.csv` | XSS-атака в именах | UTF-8 | `;` | Нет |
| `empty.csv` | Пустой файл | - | - | - |
| `invalid.csv` | Только числа (невалидные имена) | UTF-8 | - | Нет |

## Использование

На странице курса нажмите "Import from CSV" и загрузите нужный файл.

Маппинг колонок:
- `students_basic.csv`: Full Name = 0, Email = 1, Has Header = No
- `students_with_header.csv`: Full Name = 0, Email = 1, Has Header = Yes
- `students_with_group.csv`: Full Name = 0, Email = 1, Group = 2, Has Header = Yes
- Остальные: Full Name = 0, Email = 1, Has Header = No
