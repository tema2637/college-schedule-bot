#!/usr/bin/env python3
"""
corrections_parser.py
Парсер корректировок расписания из Excel (.xlsx) → changes.json

Вход : расписание.xlsx (лист с корректировками)
Выход: changes.json (список изменений по дням и группам)

Формат Excel:
  - Строки с "КОРРЕКТИРОВКА" — начало нового дня
  - "на DD.MM.YYYY (ДеньНедели)" — дата
  - Таблица: Группа | Отменённый предмет | Отменённый препод | Новый предмет | Новый препод | Номер пары | Примечание
"""

import csv
import io
import json
import re
import sys
from pathlib import Path

try:
    import openpyxl
except ImportError:
    print("Установите openpyxl: pip install openpyxl", file=sys.stderr)
    sys.exit(1)


# ─────────────────────────────────────────────────────────────
# Конвертация .xlsx → CSV-текст
# ─────────────────────────────────────────────────────────────

def xlsx_to_csv_text(xlsx_path: str) -> str:
    """Читает активный лист .xlsx и возвращает CSV-текст (cp1251, разделитель ';')."""
    wb = openpyxl.load_workbook(xlsx_path, data_only=True)
    ws = wb.active

    output = io.StringIO()
    writer = csv.writer(output, delimiter=";", lineterminator="\n")
    for row in ws.iter_rows(values_only=True):
        out = [str(cell) if cell is not None else "" for cell in row]
        writer.writerow(out)

    return output.getvalue()


# ─────────────────────────────────────────────────────────────
# Парсер корректировок (из пользовательского кода)
# ─────────────────────────────────────────────────────────────

def _detect_delimiter(sample: str) -> str:
    """Определяет разделитель CSV: запятая или точка с запятой.
    Берём строку с максимальным количеством разделителей."""
    lines = sample.strip().splitlines()
    if not lines:
        return ","
    best_semi = 0
    best_comma = 0
    for line in lines:
        semi = line.count(";")
        comma = line.count(",")
        if semi > best_semi:
            best_semi = semi
        if comma > best_comma:
            best_comma = comma
    return ";" if best_semi >= best_comma else ","


def parse_schedule_corrections(raw_csv_text: str) -> list[dict]:
    """
    Парсит CSV-текст с корректировками расписания.
    Возвращает список дней с изменениями.
    """
    result = []
    current_day = None

    f = io.StringIO(raw_csv_text.strip())
    delimiter = _detect_delimiter(raw_csv_text)
    reader = csv.reader(f, delimiter=delimiter)

    for row in reader:
        if not row:
            continue

        row = [cell.strip() for cell in row]
        line_content = "".join(row)

        # 1. Ищем шапку и парсим Дату и День недели
        if "КОРРЕКТИРОВКА" in line_content:
            current_day = {"date": "", "day_of_week": "", "changes": []}
            result.append(current_day)
            continue

        if current_day and line_content.startswith("на "):
            date_match = re.search(r"(\d{2}\.\d{2}\.\d{4})", line_content)
            day_match = re.search(r"\((.*?)\)", line_content)

            if date_match:
                d, m, y = date_match.group(1).split(".")
                current_day["date"] = f"{y}-{m}-{d}"
                if day_match:
                    current_day["day_of_week"] = day_match.group(1).lower()
            continue

        if not row or row[0] == "":
            continue
        if len(row) < 2:
            continue
        if "Группа" in row[0] or "Предмет" in row[1]:
            continue

        # 2. Обработка данных
        group = row[0]

        if "ЭКЗАМЕН" in line_content:
            current_day["changes"].append({
                "group": group,
                "lesson_number": None,
                "type": "status_change",
                "subject": "ЭКЗАМЕН",
                "teacher": None,
                "note": "ЭКЗАМЕН",
            })
            continue

        lesson_raw = row[5] if len(row) > 5 else ""
        lesson_match = re.search(r"(\d+)", lesson_raw)
        lesson_number = int(lesson_match.group(1)) if lesson_match else None

        rem_subject = row[1] if len(row) > 1 else ""
        rem_teacher = row[2] if len(row) > 2 else ""
        add_subject = row[3] if len(row) > 3 else ""
        add_teacher = row[4] if len(row) > 4 else ""
        note = row[6] if len(row) > 6 else ""

        # Логика определения типа особенности
        is_removal_only = "снять" in note.lower() or (rem_subject and not add_subject)

        # Атомарное действие 1: Пару сняли
        if rem_subject:
            current_day["changes"].append({
                "group": group,
                "lesson_number": lesson_number,
                "type": "removed",
                "subject": rem_subject,
                "teacher": rem_teacher if rem_teacher else None,
                "note": note if is_removal_only else "Замена",
            })

        # Атомарное действие 2: Пару добавили (ввели)
        if add_subject:
            current_day["changes"].append({
                "group": group,
                "lesson_number": lesson_number,
                "type": "added",
                "subject": add_subject,
                "teacher": add_teacher if add_teacher else None,
                "note": note if note else "Добавление",
            })

    return result


# ─────────────────────────────────────────────────────────────
# Преобразование в формат для рассылки (совместим с Go changes)
# ─────────────────────────────────────────────────────────────

def convert_to_broadcast_format(days: list[dict]) -> list[dict]:
    """
    Преобразует результат parse_schedule_corrections в плоский список
    изменений, готовый для интеграции с ботом.
    """
    flat = []
    for day in days:
        for change in day["changes"]:
            entry = {
                "date": day["date"],
                "day_of_week": day["day_of_week"],
                "group": change["group"],
                "lesson_number": change["lesson_number"],
                "type": change["type"],
                "subject": change["subject"],
                "teacher": change["teacher"],
                "note": change["note"],
            }
            flat.append(entry)
    return flat


# ─────────────────────────────────────────────────────────────
# main
# ─────────────────────────────────────────────────────────────

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Парсер корректировок расписания из Excel")
    parser.add_argument("--input-xlsx", default="расписание", help="Входной Excel-файл с корректировками (авто: .xlsx/.xls)")
    parser.add_argument("--output-json", default="changes.json", help="Выходной JSON-файл")
    parser.add_argument("--flat", action="store_true", default=True,
                        help="Плоский список (удобно для Go-бота)")
    args = parser.parse_args()

    xlsx_path = args.input_xlsx
    json_path = args.output_json

    # Автопоиск: сначала точное имя, потом .xlsx, потом .xls
    if not Path(xlsx_path).exists():
        for ext in ["", ".xlsx", ".xls"]:
            candidate = xlsx_path + ext
            if Path(candidate).exists():
                xlsx_path = candidate
                break
        else:
            print(f"✗ Файл не найден: {xlsx_path} (.xlsx/.xls не обнаружены)", file=sys.stderr)
            sys.exit(2)

    print(f"▶ Чтение Excel: {xlsx_path}")
    csv_text = xlsx_to_csv_text(xlsx_path)
    print(f"  Строк CSV: {len(csv_text.splitlines())}")

    print("▶ Парсинг корректировок...")
    days = parse_schedule_corrections(csv_text)

    if not days:
        print("✗ Корректировки не найдены", file=sys.stderr)
        sys.exit(1)

    print(f"  Дней с корректировками: {len(days)}")
    total_changes = sum(len(d["changes"]) for d in days)
    print(f"  Всего изменений: {total_changes}")

    if args.flat:
        output = convert_to_broadcast_format(days)
    else:
        output = days

    with open(json_path, "w", encoding="utf-8") as fh:
        json.dump(output, fh, ensure_ascii=False, indent=2)

    print(f"✔ Сохранено → {json_path}  ({len(output)} записей)")

    # Статистика
    groups = set()
    for d in days:
        for c in d["changes"]:
            groups.add(c["group"])
    print(f"  Затронуто групп: {len(groups)} → {sorted(groups)}")


if __name__ == "__main__":
    main()
