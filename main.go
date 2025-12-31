package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lu4p/unipdf/v3/extractor"
	pdf "github.com/lu4p/unipdf/v3/model"
)

func main() {
	// 命令行参数
	inputDir := flag.String("input", "", "输入PDF文件目录（必需）")
	outputDir := flag.String("output", "", "输出TXT文件目录（可选，默认与输入目录相同）")
	flag.Parse()

	// 检查必需参数
	if *inputDir == "" {
		fmt.Println("错误：必须指定输入目录")
		fmt.Println("使用方法: pdf2txt -input <PDF目录> [-output <TXT目录>]")
		os.Exit(1)
	}

	// 如果未指定输出目录，使用输入目录
	if *outputDir == "" {
		*outputDir = *inputDir
	}

	// 确保输出目录存在
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("创建输出目录失败: %v\n", err)
		os.Exit(1)
	}

	// 遍历输入目录下的所有PDF文件
	err := filepath.Walk(*inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理PDF文件
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".pdf") {
			if err := convertPDFToText(path, *outputDir); err != nil {
				fmt.Printf("转换失败 %s: %v\n", path, err)
			} else {
				fmt.Printf("转换成功: %s\n", path)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("遍历目录失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n批量转换完成！")
}

// convertPDFToText 将单个PDF文件转换为文本文件
func convertPDFToText(pdfPath string, outputDir string) error {
	// 打开PDF文件
	f, err := os.Open(pdfPath)
	if err != nil {
		return fmt.Errorf("打开PDF文件失败: %w", err)
	}
	defer f.Close()

	// 创建PDF阅读器
	pdfReader, err := pdf.NewPdfReader(f)
	if err != nil {
		return fmt.Errorf("创建PDF阅读器失败: %w", err)
	}

	// 获取页数
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return fmt.Errorf("获取页数失败: %w", err)
	}

	// 提取所有页面的文本
	var allText strings.Builder
	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return fmt.Errorf("获取第%d页失败: %w", i, err)
		}

		ex, err := extractor.New(page)
		if err != nil {
			return fmt.Errorf("创建提取器失败（第%d页）: %w", i, err)
		}

		text, err := ex.ExtractText()
		if err != nil {
			return fmt.Errorf("提取文本失败（第%d页）: %w", i, err)
		}

		allText.WriteString(text)
		allText.WriteString("\n")
	}

	// 生成输出文件名
	baseName := filepath.Base(pdfPath)
	txtFileName := strings.TrimSuffix(baseName, filepath.Ext(baseName)) + ".txt"
	outputPath := filepath.Join(outputDir, txtFileName)

	// 写入文件
	err = os.WriteFile(outputPath, []byte(allText.String()), 0644)
	if err != nil {
		return fmt.Errorf("写入TXT文件失败: %w", err)
	}

	return nil
}
