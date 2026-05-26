// Package tools содержит обёртки для запуска Python-скриптов из Go.
package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PythonRunner управляет вызовом Python-скриптов.
type PythonRunner struct {
	pythonCmd string   // "python" или "python3"
	toolsDir  string   // директория со скриптами
	workDir   string   // рабочая директория бота
}

// NewPythonRunner пытается найти подходящий Python.
func NewPythonRunner(toolsDir, workDir string) *PythonRunner {
	python := "python"
	if runtime.GOOS != "windows" {
		// На Linux/macOS обычно python3
		if _, err := exec.LookPath("python3"); err == nil {
			python = "python3"
		}
	}
	return &PythonRunner{
		pythonCmd: python,
		toolsDir:  toolsDir,
		workDir:   workDir,
	}
}

// XlsxToCsv конвертирует Excel-файл в CSV.
func (r *PythonRunner) XlsxToCsv(xlsxPath, csvPath string) error {
	script := filepath.Join(r.toolsDir, "xlsx_to_csv.py")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("скрипт конвертера не найден: %w", err)
	}
	cmd := exec.Command(r.pythonCmd, script, xlsxPath, csvPath)
	cmd.Dir = r.workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("конвертация xlsx→csv не удалась: %w\n%s", err, string(out))
	}
	return nil
}

// ParseSchedule запускает schedule_parser.py для генерации schedule.json.
func (r *PythonRunner) ParseSchedule(csvPath, jsonPath string) error {
	script := filepath.Join(".", "schedule_parser.py")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("parser не найден: %w", err)
	}
	cmd := exec.Command(r.pythonCmd, script,
		"--input-csv", csvPath,
		"--output-json", jsonPath,
	)
	cmd.Dir = r.workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("парсер вернул ошибку: %w\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Сохранено") {
		return fmt.Errorf("парсер не сохранил JSON:\n%s", string(out))
	}
	return nil
}

// FullPipeline: xlsx → csv → schedule.json + changes.json.
func (r *PythonRunner) FullPipeline(xlsxPath, csvPath, jsonPath, changesJsonPath string) (string, error) {
	var sb strings.Builder

	// --- Расписание ---
	sb.WriteString("▶ Конвертация Excel → CSV...\n")
	if err := r.XlsxToCsv(xlsxPath, csvPath); err != nil {
		return sb.String(), err
	}
	time.Sleep(200 * time.Millisecond)

	sb.WriteString("▶ Парсинг CSV → JSON...\n")
	if err := r.ParseSchedule(csvPath, jsonPath); err != nil {
		return sb.String(), err
	}
	sb.WriteString("  ✅ Расписание → " + jsonPath + "\n")

	// --- Корректировки ---
	sb.WriteString("▶ Парсинг корректировок...\n")
	if err := r.ParseCorrections(xlsxPath, changesJsonPath); err != nil {
		// Корректировки — не критично, логируем и продолжаем
		sb.WriteString(fmt.Sprintf("  ⚠️ Корректировки не спарсились: %v\n", err))
	} else {
		sb.WriteString("  ✅ Корректировки → " + changesJsonPath + "\n")
	}

	sb.WriteString("✅ Готово! Всё обновлено.")
	return sb.String(), nil
}

// ParseCorrections запускает corrections_parser.py для генерации changes.json из xlsx.
func (r *PythonRunner) ParseCorrections(xlsxPath, jsonPath string) error {
	script := filepath.Join(r.toolsDir, "corrections_parser.py")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("скрипт парсера корректировок не найден: %w", err)
	}

	cmd := exec.Command(r.pythonCmd, script,
		"--input-xlsx", xlsxPath,
		"--output-json", jsonPath,
	)
	cmd.Dir = r.workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("парсер корректировок вернул ошибку: %w\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Сохранено") {
		return fmt.Errorf("парсер корректировок не сохранил JSON:\n%s", string(out))
	}
	return nil
}
