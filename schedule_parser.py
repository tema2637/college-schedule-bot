"""
Парсер расписания ГБПОУ СКС  ―  schedule_parser.py
====================================================
Вход : Расписание-2.csv  (cp1251, разделитель ';')
Выход: schedule.json

Алгоритм
--------
1. Из всего файла собираем мастер-список преподавателей
   (полные ФИО + только фамилии).
2. Для каждой ячейки (группа × день × пара):
   a. Нормализуем текст (убираем слипшиеся токены).
   b. Находим все позиции преподавателей → делим текст на сегменты.
   c. Каждый сегмент → одно занятие:
      ‑ ПОСЛЕДНЯЯ скобка «только цифры» = недели.
      ‑ Скобка с буквами (1и2, п/гр …) = квалификатор предмета.
      ‑ 3-значные числа (не внутри МДК.XX.XX) = аудитория.
      ‑ Всё остальное = название предмета.
   d. Если сегмент перед преподавателем пуст/содержит только аудиторию
      → подгруппа: предмет и недели наследуются от предыдущего сегмента.
3. Валидируем результат (пустые предметы, пустые недели) и сохраняем JSON.
"""

import csv
import re
import json
import sys
import argparse
from pathlib import Path
from collections import defaultdict

# ─────────────────────────────────────────────────────────────
# Конфигурация (умолчания, можно переопределить через аргументы)
# ─────────────────────────────────────────────────────────────
DEFAULT_CSV   = "Расписание-2.csv"
DEFAULT_JSON  = "schedule.json"

GROUPS_ROW = 4    # строка (0-based) с заголовком групп
GROUPS_COL = 2    # первый столбец с группой

DAYS = ["ПОНЕДЕЛЬНИК", "ВТОРНИК", "СРЕДА", "ЧЕТВЕРГ", "ПЯТНИЦА"]

# Расписание звонков — разное для разных дней недели
# Ключ внешнего dict: day_num (1-based, 1=ПН … 5=ПТ)
# Ключ внутреннего dict: номер пары
TIME_SLOTS: dict[int, dict[int, tuple[str, str]]] = {
    1: {  # ПОНЕДЕЛЬНИК
        1: ("09:10", "10:40"),
        2: ("10:50", "12:20"),
        3: ("12:50", "14:20"),
        4: ("14:30", "16:00"),
        5: ("16:10", "17:40"),
        6: ("17:50", "19:20"),
    },
    2: {  # ВТОРНИК
        1: ("08:30", "10:00"),
        2: ("10:10", "11:40"),
        3: ("12:10", "13:40"),
        4: ("13:50", "15:20"),
        5: ("15:30", "17:00"),
        6: ("17:10", "18:40"),
        7: ("18:50", "20:20"),
    },
    3: {  # СРЕДА (совпадает с вторником/пятницей)
        1: ("08:30", "10:00"),
        2: ("10:10", "11:40"),
        3: ("12:10", "13:40"),
        4: ("13:50", "15:20"),
        5: ("15:30", "17:00"),
        6: ("17:10", "18:40"),
        7: ("18:50", "20:20"),
    },
    4: {  # ЧЕТВЕРГ (орг.час после 2-й пары, 3-я пара сдвинута)
        1: ("08:30", "10:00"),
        2: ("10:10", "11:40"),
        3: ("13:00", "14:30"),
        4: ("14:40", "16:10"),
        5: ("16:20", "17:50"),
        6: ("18:00", "19:30"),
    },
    5: {  # ПЯТНИЦА (совпадает с вторником/средой)
        1: ("08:30", "10:00"),
        2: ("10:10", "11:40"),
        3: ("12:10", "13:40"),
        4: ("13:50", "15:20"),
        5: ("15:30", "17:00"),
        6: ("17:10", "18:40"),
        7: ("18:50", "20:20"),
    },
}

# ─────────────────────────────────────────────────────────────
# Регулярные выражения
# ─────────────────────────────────────────────────────────────

# ФИО: Фамилия И.О. (второй инициал необязателен)
RE_TEACHER_FULL = re.compile(
    r'[А-ЯЁ][а-яё]+(?:-[А-ЯЁ][а-яё]+)?\s+[А-ЯЁ]\.\s?[А-ЯЁ]?\.?'
)

# Одиночная фамилия (заглавная + ≥3 строчных)
RE_SURNAME_WORD = re.compile(r'\b([А-ЯЁ][а-яё]{3,})\b')

# Скобки с ТОЛЬКО цифрами/запятыми/дефисами/точками/пробелами
# (минимум одна цифра) = НЕДЕЛИ
RE_WEEKS_PAREN = re.compile(r'\((\d[\d,.\-\s]*)\)')

# Аудитория: 3 цифры + необязательная строчная буква,
# НЕ внутри МДК-номера вида XX.XX
RE_ROOM = re.compile(r'(?<![.\d])(\d{3}[а-яё]?)(?!\.\d)(?!\d)')


# ─────────────────────────────────────────────────────────────
# Чтение файла
# ─────────────────────────────────────────────────────────────

def read_csv(path: str) -> list[list[str]]:
    with open(path, encoding="cp1251", newline="") as fh:
        return list(csv.reader(fh, delimiter=";"))


# ─────────────────────────────────────────────────────────────
# Мастер-список преподавателей
# ─────────────────────────────────────────────────────────────

# Слова, которые выглядят как фамилии, но являются частью названий предметов
_TEACHER_STOPWORDS = {
    "Рус", "Ин", "МДК", "ОБП", "ЗР", "ТВ", "МС", "ОП", "ФГ",
    "ДМ", "СД", "ТДВ", "ИТ", "ПД", "СС", "БЖ", "КС", "ОФГ",
    "СКС", "ОПБД", "ПОПД", "КГ", "АиЦУ",
    "Физ", "Прак", "Осн",
}


def build_teacher_master(rows: list[list[str]]) -> tuple[set[str], set[str]]:
    """Возвращает (полные_ФИО, фамилии)."""
    all_text = " ".join(cell for row in rows for cell in row)

    full_names: set[str] = set()
    for m in RE_TEACHER_FULL.finditer(all_text):
        full_names.add(m.group(0).strip())

    surnames: set[str] = set()
    for name in full_names:
        sn = name.split()[0].rstrip(".,")
        if len(sn) >= 4 and sn not in _TEACHER_STOPWORDS:
            surnames.add(sn)

    return full_names, surnames


# ─────────────────────────────────────────────────────────────
# Предобработка текста
# ─────────────────────────────────────────────────────────────

def preprocess(text: str) -> str:
    # "412Обществоз" → "412 Обществоз"  (цифра + заглавная)
    text = re.sub(r"(\d)([А-ЯЁ])", r"\1 \2", text)
    # "Е.А.228" → "Е.А. 228"
    # Используем (?<!\w) чтобы НЕ разбивать МДК.02 (К - часть слова)
    text = re.sub(r"(?<!\w)([А-ЯЁ]\.)(\d)", r"\1 \2", text)
    # "(1-5)Слово" → "(1-5) Слово"
    text = re.sub(r"\)([А-ЯЁа-яё])", r") \1", text)
    # "Фамилия303" → "Фамилия 303"  (строчная + 3 цифры)
    text = re.sub(r"([а-яё])(\d{3})", r"\1 \2", text)
    return " ".join(text.split())


# ─────────────────────────────────────────────────────────────
# Разбор недель
# ─────────────────────────────────────────────────────────────

def parse_weeks(s: str) -> list[int]:
    """'1-5,7,9-11.' → [1,2,3,4,5,7,9,10,11]"""
    result: set[int] = set()
    cleaned = re.sub(r"[^\d\-,]", "", s)
    for part in cleaned.split(","):
        part = part.strip()
        if not part:
            continue
        if "-" in part:
            try:
                a, b = map(int, part.split("-", 1))
                result.update(range(a, b + 1))
            except ValueError:
                pass
        else:
            try:
                result.add(int(part))
            except ValueError:
                pass
    return sorted(result)


# ─────────────────────────────────────────────────────────────
# Поиск преподавателей в тексте
# ─────────────────────────────────────────────────────────────

def find_teachers(
    text: str,
    full_names: set[str],
    surnames: set[str],
) -> list[tuple[int, int, str]]:
    """
    Возвращает [(start, end, teacher_str), …], отсортированный по позиции.
    Перекрытия не допускаются (первое совпадение «занимает» символы).
    """
    used = bytearray(len(text))  # 0 = свободно, 1 = занято
    found: list[tuple[int, int, str]] = []

    # 1. Полные ФИО (приоритет — берём как можно раньше)
    for m in RE_TEACHER_FULL.finditer(text):
        s, e = m.start(), m.end()
        if any(used[s:e]):
            continue
        used[s:e] = b"\x01" * (e - s)
        found.append((s, e, m.group(0).strip()))

    # 2. Одиночные фамилии из мастер-списка
    for m in RE_SURNAME_WORD.finditer(text):
        sn = m.group(1)
        if sn not in surnames:
            continue
        s, e = m.start(), m.end()
        if any(used[s:e]):
            continue
        # Не берём фамилию, если сразу за ней стоят инициалы
        # (значит RE_TEACHER_FULL её не поймал только из-за нестандартного
        # разделителя — обрабатываем как ФИО)
        after = text[e : e + 6]
        if re.match(r"\s+[А-ЯЁ]\.", after):
            continue
        used[s:e] = b"\x01" * (e - s)
        found.append((s, e, sn))

    found.sort(key=lambda x: x[0])
    return found


# ─────────────────────────────────────────────────────────────
# Извлечение предмета, недель, аудитории из сегмента
# ─────────────────────────────────────────────────────────────

def extract_from_segment(seg: str) -> tuple[str, list[int], str]:
    """
    Из куска текста перед преподавателем извлекаем:
      (subject, weeks, room)

    Правило недели: ПОСЛЕДНЯЯ скобка «только цифры» = недели.
    Скобки с буквами (1и2, п/гр …) = часть названия предмета.
    Первое трёхзначное число = аудитория.
    """
    # Найдём все «цифровые» скобки
    digit_parens = list(RE_WEEKS_PAREN.finditer(seg))

    weeks: list[int] = []
    weeks_span: tuple[int, int] | None = None

    if digit_parens:
        last = digit_parens[-1]
        weeks = parse_weeks(last.group(1))
        weeks_span = (last.start(), last.end())

    # Маска: удаляем недели и аудиторию, чтобы получить предмет
    mask = list(seg)

    if weeks_span:
        for i in range(*weeks_span):
            mask[i] = "\0"

    # Аудитория — первое подходящее 3-значное число
    room = ""
    for m in RE_ROOM.finditer(seg):
        # Пропускаем позиции, уже съеденные неделями
        if all(mask[i] == "\0" for i in range(m.start(), m.end())):
            continue
        room = m.group(1)
        for i in range(m.start(), m.end()):
            mask[i] = "\0"
        break

    # Собираем предмет из оставшегося текста
    raw = "".join(c for c in mask if c != "\0")
    subject = raw.strip(" ;/,-").strip()
    subject = " ".join(subject.split())

    return subject, weeks, room


# ─────────────────────────────────────────────────────────────
# Проверка «подгрупп» (сегмент только с аудиторией или пустой)
# ─────────────────────────────────────────────────────────────

def is_subgroup_segment(seg: str) -> bool:
    """
    True, если сегмент не содержит полноценного занятия (нет недель, нет предмета),
    то есть является продолжением предыдущей подгруппы.
    """
    clean = seg.strip(" /;.,")
    if not clean:
        return True
    # Только аудитория (3 цифры + буква) + возможные слэши/пробелы
    return bool(re.fullmatch(r"\d{3}[а-яё]?", clean))


# ─────────────────────────────────────────────────────────────
# Основной парсер ячейки
# ─────────────────────────────────────────────────────────────

def parse_cell(
    raw_text: str,
    group_name: str,
    day_num: int,       # 1-based
    lesson_num: int,    # 1-based (номер пары)
    full_names: set[str],
    surnames: set[str],
) -> list[dict]:
    if not raw_text.strip():
        return []

    text = preprocess(raw_text)
    teachers = find_teachers(text, full_names, surnames)

    if not teachers:
        # Нет преподавателей — пробуем вытащить хоть что-то
        subj, weeks, room = extract_from_segment(text)
        if weeks and subj:
            return [_make(subj, "", room, weeks, group_name, day_num, lesson_num)]
        return []

    lessons: list[dict] = []
    # Базовый предмет/недели для наследования подгруппами
    base_subject = ""
    base_weeks: list[int] = []
    prev_end = 0

    for t_start, t_end, t_name in teachers:
        seg = text[prev_end:t_start]
        prev_end = t_end

        if is_subgroup_segment(seg) and base_subject and base_weeks:
            # ── Подгруппа ───────────────────────────────────────────
            room = ""
            m = RE_ROOM.search(seg.strip(" /;.,"))
            if m:
                room = m.group(1)
            lessons.append(_make(base_subject, t_name, room, base_weeks,
                                 group_name, day_num, lesson_num))
        else:
            # ── Обычное занятие ─────────────────────────────────────
            subj, weeks, room = extract_from_segment(seg)

            if not weeks:
                # Нет недель — запоминаем предмет как возможную базу и пропускаем
                if subj:
                    base_subject = subj
                continue

            if not subj:
                subj = base_subject   # берём предмет от предыдущего сегмента

            lessons.append(_make(subj, t_name, room, weeks,
                                 group_name, day_num, lesson_num))
            base_subject = subj
            base_weeks = weeks

    return lessons


def _make(subject, teacher, room, weeks, group, day, lesson):
    day_slots = TIME_SLOTS.get(day, {})
    slot_time = day_slots.get(lesson, ("", ""))
    return {
        "LessonTitle":  subject,
        "TeacherName":  teacher,
        "Cabinet":      room,
        "Weeks":        weeks,
        "Group":        {"Name": group},
        "TimeSlot": {
            "NumberSlot": lesson,
            "DayOfWeek":  day,
            "StartTime":  slot_time[0],
            "EndTime":    slot_time[1],
        },
    }


# ─────────────────────────────────────────────────────────────
# Сбор «сырых» текстов ячеек
# ─────────────────────────────────────────────────────────────

def collect_cells(
    rows: list[list[str]],
    num_groups: int,
) -> dict[tuple[int, int, int], str]:
    """
    Объединяет строки расписания в ячейки (day, lesson, group_idx) → текст.
    Несколько строк одного слота склеиваются пробелом.
    """
    cells: dict[tuple[int, int, int], list[str]] = defaultdict(list)
    cur_day, cur_lesson = None, None

    for row in rows[GROUPS_ROW + 1:]:
        if not row:
            continue

        day_str = row[0].strip().upper()
        if day_str in DAYS:
            cur_day = DAYS.index(day_str)

        if len(row) > 1 and row[1].strip().isdigit():
            cur_lesson = int(row[1].strip())

        if cur_day is None or cur_lesson is None:
            continue

        for gi in range(num_groups):
            ci = gi + GROUPS_COL
            if ci >= len(row):
                continue
            cell = row[ci].strip()
            if cell:
                cells[(cur_day, cur_lesson, gi)].append(cell)

    return {k: " ".join(" ".join(v).split()) for k, v in cells.items()}


# ─────────────────────────────────────────────────────────────
# Валидация
# ─────────────────────────────────────────────────────────────

def validate(lessons: list[dict]) -> list[str]:
    issues: list[str] = []
    for r in lessons:
        g   = r["Group"]["Name"]
        d   = r["TimeSlot"]["DayOfWeek"]
        sl  = r["TimeSlot"]["NumberSlot"]
        tag = f"[{DAYS[d-1]} п{sl} {g}]"

        if not r["LessonTitle"].strip():
            issues.append(f"{tag} пустой предмет  teacher={r['TeacherName']!r}")
        if not r["Weeks"]:
            issues.append(f"{tag} пустые недели   subj={r['LessonTitle']!r}")
        if re.match(r"^\d{4,}", r["LessonTitle"]):
            issues.append(f"{tag} предмет начинается с длинного числа: {r['LessonTitle']!r}")
        if len(r["LessonTitle"]) > 100:
            issues.append(f"{tag} подозрительно длинный предмет ({len(r['LessonTitle'])} симв)")
    return issues


# ─────────────────────────────────────────────────────────────
# main
# ─────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Парсер расписания ГБПОУ СКС")
    parser.add_argument("--input-csv",  default=DEFAULT_CSV,  help="Входной CSV-файл")
    parser.add_argument("--output-json", default=DEFAULT_JSON, help="Выходной JSON-файл")
    args = parser.parse_args()

    csv_path = args.input_csv
    json_path = args.output_json
    
    if not Path(csv_path).exists():
        print(f"✗ Файл не найден: {csv_path}")
        sys.exit(2)

    print(f"▶ Чтение файла: {csv_path} …")
    rows = read_csv(csv_path)
    print(f"  Строк: {len(rows)}")

    # Группы
    groups_row = rows[GROUPS_ROW]
    groups = [
        groups_row[ci].strip()
        for ci in range(GROUPS_COL, len(groups_row))
    ]
    print(f"  Групп: {len(groups)}")

    # Мастер-список преподавателей
    print("▶ Построение списка преподавателей …")
    full_names, surnames = build_teacher_master(rows)
    print(f"  ФИО: {len(full_names)}, фамилий: {len(surnames)}")

    # Сбор ячеек
    print("▶ Сбор ячеек …")
    cells = collect_cells(rows, len(groups))
    print(f"  Ячеек с данными: {len(cells)}")

    # Парсинг
    print("▶ Парсинг …")
    all_lessons: list[dict] = []
    parse_errors: list[str] = []

    for (day_idx, lesson_num, gi), text in sorted(cells.items()):
        group_name = groups[gi]
        try:
            records = parse_cell(
                text, group_name,
                day_idx + 1, lesson_num,
                full_names, surnames,
            )
            all_lessons.extend(records)
        except Exception as exc:
            parse_errors.append(
                f"[{DAYS[day_idx]} п{lesson_num} {group_name}] "
                f"{type(exc).__name__}: {exc}"
            )

    # Сортировка
    all_lessons.sort(key=lambda r: (
        r["TimeSlot"]["DayOfWeek"],
        r["TimeSlot"]["NumberSlot"],
        r["Group"]["Name"],
    ))

    print(f"  Занятий извлечено: {len(all_lessons)}")

    # Валидация
    print("▶ Валидация …")
    issues = validate(all_lessons)
    if issues:
        print(f"  ⚠  Предупреждений: {len(issues)}")
        for w in issues[:30]:
            print(f"     {w}")
    else:
        print("  ✔  Нарушений не найдено")

    if parse_errors:
        print(f"  ✗  Ошибок парсинга: {len(parse_errors)}")
        for e in parse_errors[:10]:
            print(f"     {e}")
    else:
        print("  ✔  Ошибок парсинга нет")

    # Сохранение
    with open(json_path, "w", encoding="utf-8") as fh:
        json.dump(all_lessons, fh, ensure_ascii=False, indent=2)
    print(f"\n✔  Сохранено → {json_path}  ({len(all_lessons)} записей)")

    # ─── Ручная проверка примеров ────────────────────────────────────────
    def show(group, day_idx):
        print(f"\n{'─'*60}")
        print(f"  Группа: {group}  /  {DAYS[day_idx]}")
        print(f"{'─'*60}")
        for r in all_lessons:
            if r["Group"]["Name"] == group and r["TimeSlot"]["DayOfWeek"] == day_idx + 1:
                wks = r["Weeks"]
                wk_str = str(wks[:5])[:-1] + ("…]" if len(wks) > 5 else "]")
                print(
                    f"  п{r['TimeSlot']['NumberSlot']} │ "
                    f"{r['LessonTitle']:<28s} │ "
                    f"ауд {r['Cabinet']:<5s} │ "
                    f"{r['TeacherName']:<25s} │ "
                    f"нед {wk_str}"
                )

    # Примеры только если это стандартные пути (не при встроенном вызове)
    if csv_path == DEFAULT_CSV:
        show("ИП 252",  0)   # Понедельник
        show("РЗ 252",  0)
        show("ИКС 223", 0)
        show("ИКС224",  1)   # Вторник — с МДК-подгруппами
        show("ИВ 234",  0)   # Понедельник — с Кривцова/Степаненко

    return len(all_lessons), all_lessons


if __name__ == "__main__":
    main()
