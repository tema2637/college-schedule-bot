#!/usr/bin/env python3
"""
xlsx_to_csv.py
Конвертер Excel → CSV для schedule_parser.py
Вход : .xlsx (любой лист)
Выход: Расписание-2.csv (cp1251, разделитель ';')
Использование:
    python xlsx_to_csv.py input.xlsx [output.csv]
"""

import sys
import csv
from pathlib import Path

try:
    import openpyxl
except ImportError:
    print("Установите openpyxl: pip install openpyxl")
    sys.exit(1)


def xlsx_to_csv(xlsx_path: str, csv_path: str) -> None:
    wb = openpyxl.load_workbook(xlsx_path, data_only=True)
    ws = wb.active

    with open(csv_path, "w", newline="", encoding="cp1251") as fh:
        writer = csv.writer(fh, delimiter=";", lineterminator="\n")
        for row in ws.iter_rows(values_only=True):
            # Пустые ячейки → пустые строки
            out = [str(cell) if cell is not None else "" for cell in row]
            writer.writerow(out)

    print(f"Конвертация завершена: {xlsx_path} → {csv_path}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Использование: python xlsx_to_csv.py input.xlsx [output.csv]")
        sys.exit(1)

    xlsx_file = sys.argv[1]
    csv_file = sys.argv[2] if len(sys.argv) > 2 else "Расписание-2.csv"

    if not Path(xlsx_file).exists():
        print(f"Файл не найден: {xlsx_file}")
        sys.exit(1)

    xlsx_to_csv(xlsx_file, csv_file)
    print(f"Готово: {csv_file}")
