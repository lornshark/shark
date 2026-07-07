package test

import (
	"encoding/csv"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lornshark/shark/sharkcsv"
	"github.com/lornshark/shark/sharkfunc"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type TestUser struct {
	ID       int       `csv:"id" gorm:"column:id" json:"id"`
	Name     string    `csv:"name"`
	Score    float64   `csv:"score"`
	IsActive bool      `csv:"is_active"`
	Age      *int      `csv:"age"`
	JoinedAt time.Time `csv:"joined_at"`
}

func TestSharkCsv_Scan(t *testing.T) {
	// 创建临时文件并填充测试数据
	tempFile, err := os.CreateTemp("", "test_shark_csv_*.csv")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 写入 header 和 2005 条数据
	writer := csv.NewWriter(tempFile)
	header := []string{"id", "name", "score", "is_active", "age", "joined_at"}
	if err := writer.Write(header); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	for i := 1; i <= 2005; i++ {
		ageStr := fmt.Sprintf("%d", i+20)
		if i%2 == 0 {
			ageStr = "" // 空白测试指针 null
		}
		row := []string{
			fmt.Sprintf("%d", i),
			fmt.Sprintf("User%d", i),
			fmt.Sprintf("%.2f", float64(i)*1.1),
			fmt.Sprintf("%t", i%2 != 0),
			ageStr,
			now.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			t.Fatalf("failed to write row: %v", err)
		}
	}
	writer.Flush()
	tempFile.Close()

	// 1. 新建并校验构造器
	opts := &sharkcsv.SharkCsvOption{
		Delimiter: sharkfunc.Pointer(rune(',')),
		HasHeader: sharkfunc.Pointer(true),
		Encoding:  sharkfunc.Pointer("UTF-8"),
	}
	sc := sharkcsv.OpenFile(tempFile.Name(), opts)
	if sc.Delimiter != ',' || !sc.HasHeader || sc.Encoding != "UTF-8" {
		t.Fatalf("incorrect config values from NewSharkCsv")
	}

	// 2. 验证 Callback 模式（分 1000 条流式读取）
	t.Run("Callback Mode - Standard struct", func(t *testing.T) {
		var batches [][]TestUser
		err := sc.Scan(func(batch []TestUser) error {
			batches = append(batches, batch)
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected Scan error: %v", err)
		}

		if len(batches) != 3 {
			t.Fatalf("expected 3 batches, got %d", len(batches))
		}
		if len(batches[0]) != 1000 || len(batches[1]) != 1000 || len(batches[2]) != 5 {
			t.Fatalf("batch sizes incorrect: %d, %d, %d", len(batches[0]), len(batches[1]), len(batches[2]))
		}

		// 检查部分数据强转
		u1 := batches[0][0]
		if u1.ID != 1 || u1.Name != "User1" || u1.Score != 1.1 || !u1.IsActive || u1.Age == nil || *u1.Age != 21 || !u1.JoinedAt.Equal(now) {
			t.Fatalf("data conversion error on index 0: %+v", u1)
		}
		u2 := batches[0][1]
		if u2.Age != nil {
			t.Fatalf("expected nil age for even user index, got %d", *u2.Age)
		}
	})

	// 3. 验证 Slice 指针模式
	t.Run("Pointer Mode - Standard list", func(t *testing.T) {
		var users []TestUser
		err := sc.Scan(&users)
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

	// 4. 验证直接返回 bool 切断机制
	t.Run("Callback Bool Abort", func(t *testing.T) {
		count := 0
		err := sc.Scan(func(batch []TestUser) bool {
			count += len(batch)
			return false // 第一批直接中断
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1000 {
			t.Fatalf("expected to stop after 1000 rows, got %d", count)
		}
	})

	// 5. 验证原始 []string 无 Header 反序列化
	t.Run("Raw String Slice Callbacks", func(t *testing.T) {
		// 无 Header 时
		scNoHeader := &sharkcsv.SharkCsv{
			FilePath:  sc.FilePath,
			Delimiter: ',',
			HasHeader: false,
			Encoding:  "UTF-8",
		}
		batches := 0
		err := scNoHeader.Scan(func(batch [][]string) error {
			batches++
			if batches == 1 {
				// 第一行的第一条记录应该是有 Header 信息的（因为把 header 也当数据读了）
				if batch[0][0] != "id" {
					t.Fatalf("expected first element to be 'id', got %s", batch[0][0])
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed scanning raw slice: %v", err)
		}
	})
}

func TestSharkCsv_GBKEncoding(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test_shark_csv_gbk_*.csv")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 采用 GBK 编码进行写入
	gbkEncoder := simplifiedchinese.GBK.NewEncoder()
	writer := csv.NewWriter(transform.NewWriter(tempFile, gbkEncoder))
	header := []string{"id", "姓名"}
	if err := writer.Write(header); err != nil {
		t.Fatalf("failed standard gbk csv headers write: %v", err)
	}
	if err := writer.Write([]string{"1", "张三"}); err != nil {
		t.Fatalf("failed standard gbk csv rows write: %v", err)
	}
	writer.Flush()
	tempFile.Close()

	type GBKUser struct {
		ID   int    `csv:"id"`
		Name string `csv:"姓名"`
	}

	sc := sharkcsv.OpenFile(tempFile.Name(), &sharkcsv.SharkCsvOption{
		Encoding: sharkfunc.Pointer("GBK"),
	})

	var users []GBKUser
	if err := sc.Scan(&users); err != nil {
		t.Fatalf("failed checking GBK scan: %v", err)
	}

	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Name != "张三" {
		t.Fatalf("GBK decoding name fails. expected '张三', got: %q", users[0].Name)
	}
}
