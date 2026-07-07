package test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lornshark/shark/sharkexcel"
	"github.com/lornshark/shark/sharkfunc"
	"github.com/xuri/excelize/v2"
)

type TestExcelUser struct {
	ID       int       `excel:"id"`
	Name     string    `excel:"name"`
	Score    float64   `excel:"score"`
	IsActive bool      `excel:"is_active"`
	Age      *int      `excel:"age"`
	JoinedAt time.Time `excel:"joined_at"`
}

func TestSharkExcel_Scan(t *testing.T) {
	// 1. 产生临时 excel 文件，填充 2005 条数据
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		t.Fatalf("failed to create sheet: %v", err)
	}
	f.SetActiveSheet(index)

	headers := []string{"id", "name", "score", "is_active", "age", "joined_at"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	now := time.Now().Truncate(time.Second)
	for i := 1; i <= 2005; i++ {
		rowIdx := i + 1
		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowIdx), i)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowIdx), fmt.Sprintf("User%d", i))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowIdx), float64(i)*1.1)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowIdx), i%2 != 0)
		if i%2 == 0 {
			_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowIdx), "")
		} else {
			_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowIdx), i+20)
		}
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowIdx), now.Format(time.RFC3339))
	}

	tempFile, err := os.CreateTemp("", "test_shark_excel_*.xlsx")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	if err := f.SaveAs(tempFile.Name()); err != nil {
		t.Fatalf("failed to save excel: %v", err)
	}

	// 2. 初始化 SharkExcel
	opts := &sharkexcel.SharkExcelOption{
		HasHeader: sharkfunc.Pointer(true),
		SheetName: sharkfunc.Pointer(sheetName),
	}
	se := sharkexcel.OpenFile(tempFile.Name(), opts)

	// 3. 测试 Callback 回调分批模式（每批 1000 行）
	t.Run("Callback Mode - Standard excel struct", func(t *testing.T) {
		var batches [][]TestExcelUser
		err := se.Scan(func(batch []TestExcelUser) error {
			batches = append(batches, batch)
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected excel scan error: %v", err)
		}

		if len(batches) != 3 {
			t.Fatalf("expected 3 batches, got %d", len(batches))
		}
		if len(batches[0]) != 1000 || len(batches[1]) != 1000 || len(batches[2]) != 5 {
			t.Fatalf("batch sizes incorrect: %d, %d, %d", len(batches[0]), len(batches[1]), len(batches[2]))
		}

		// 检查行转换
		u1 := batches[0][0]
		if u1.ID != 1 || u1.Name != "User1" || u1.Score != 1.1 || !u1.IsActive || u1.Age == nil || *u1.Age != 21 || !u1.JoinedAt.Equal(now) {
			t.Fatalf("data conversion error on index 0: %+v", u1)
		}
		u2 := batches[0][1]
		if u2.Age != nil {
			t.Fatalf("expected nil age for even user index, got %d", *u2.Age)
		}
	})

	// 4. 测试指针全量加载模式
	t.Run("Pointer Mode - Standard list", func(t *testing.T) {
		var users []TestExcelUser
		err := se.Scan(&users)
		if err != nil {
			t.Fatalf("unexpected Pointer Scan error: %v", err)
		}
		if len(users) != 2005 {
			t.Fatalf("expected 2005 total users, got %d", len(users))
		}
		if users[2004].ID != 2005 || users[2004].Name != "User2005" {
			t.Fatalf("unexpected last user data: %+v", users[2004])
		}
	})

	// 5. 测试 Callback Bool 中断
	t.Run("Callback Bool Abort", func(t *testing.T) {
		count := 0
		err := se.Scan(func(batch []TestExcelUser) bool {
			count += len(batch)
			return false // 取消加载剩余
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1000 {
			t.Fatalf("expected to stop after first 1000 rows, got %d", count)
		}
	})

	// 6. 测试无表头和原始文本列表
	t.Run("Raw String Slice Callbacks No Header", func(t *testing.T) {
		seNoHeader := sharkexcel.OpenFile(tempFile.Name(), &sharkexcel.SharkExcelOption{
			HasHeader: sharkfunc.Pointer(false),
			SheetName: sharkfunc.Pointer(sheetName),
		})

		batches := 0
		err := seNoHeader.Scan(func(batch [][]string) error {
			batches++
			if batches == 1 {
				if batch[0][0] != "id" {
					t.Fatalf("expected raw sheet first row as headers list, got %s", batch[0][0])
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed scanning raw slice: %v", err)
		}
	})
}
