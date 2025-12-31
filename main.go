package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lu4p/unipdf/v3/extractor"
	pdf "github.com/lu4p/unipdf/v3/model"
)

func main() {
	// 命令行参数
	inputDir := flag.String("input", "", "输入PDF文件目录（必需）")
	outputDir := flag.String("output", "", "输出TXT文件目录（可选，默认与输入目录相同）")
	webMode := flag.Bool("web", false, "启动Web服务器模式")
	port := flag.String("port", "8082", "Web服务器端口（默认8082）")
	flag.Parse()

	// Web服务器模式
	if *webMode {
		startWebServer(*port)
		return
	}

	// 命令行模式
	// 检查必需参数
	if *inputDir == "" {
		fmt.Println("错误：必须指定输入目录")
		fmt.Println("使用方法: pdf2txt -input <PDF目录> [-output <TXT目录>]")
		fmt.Println("或启动Web界面: pdf2txt -web [-port 8080]")
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

// Web服务器相关函数
func startWebServer(port string) {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/api/upload-convert", uploadConvertHandler)
	http.HandleFunc("/api/upload-save-local", uploadSaveLocalHandler)

	log.Printf("Web服务器启动在 http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("index").Parse(htmlTemplate))
	tmpl.Execute(w, nil)
}

func uploadConvertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析multipart表单，限制最大内存为100MB
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, fmt.Sprintf("解析表单失败: %v", err), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "没有上传文件", http.StatusBadRequest)
		return
	}

	// 创建ZIP缓冲区
	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)

	successCount := 0
	failedCount := 0

	// 处理每个上传的PDF文件
	for _, fileHeader := range files {
		if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".pdf") {
			continue
		}

		// 打开上传的文件
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("打开文件失败 %s: %v\n", fileHeader.Filename, err)
			failedCount++
			continue
		}

		// 转换PDF为文本
		text, err := convertPDFReaderToText(file)
		file.Close()

		if err != nil {
			log.Printf("转换失败 %s: %v\n", fileHeader.Filename, err)
			failedCount++
			continue
		}

		// 生成TXT文件名
		txtFileName := strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename)) + ".txt"

		// 添加到ZIP
		zipFile, err := zipWriter.Create(txtFileName)
		if err != nil {
			log.Printf("创建ZIP文件失败 %s: %v\n", txtFileName, err)
			failedCount++
			continue
		}

		if _, err := zipFile.Write([]byte(text)); err != nil {
			log.Printf("写入ZIP失败 %s: %v\n", txtFileName, err)
			failedCount++
			continue
		}

		successCount++
		log.Printf("转换成功: %s\n", fileHeader.Filename)
	}

	// 关闭ZIP writer
	if err := zipWriter.Close(); err != nil {
		http.Error(w, fmt.Sprintf("关闭ZIP失败: %v", err), http.StatusInternalServerError)
		return
	}

	if successCount == 0 {
		http.Error(w, "所有文件转换失败", http.StatusInternalServerError)
		return
	}

	// 返回ZIP文件
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=converted-texts.zip")
	w.Write(zipBuffer.Bytes())

	log.Printf("转换完成: 成功 %d, 失败 %d\n", successCount, failedCount)
}

func uploadSaveLocalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析multipart表单，限制最大内存为100MB
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, fmt.Sprintf("解析表单失败: %v", err), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	paths := r.MultipartForm.Value["paths"]

	if len(files) == 0 {
		http.Error(w, "没有上传文件", http.StatusBadRequest)
		return
	}

	// 确定输出目录
	outputDir := ""
	if dirs := r.MultipartForm.Value["outputDir"]; len(dirs) > 0 && dirs[0] != "" {
		outputDir = dirs[0]
	} else {
		// 默认使用桌面的 PDF转换结果 文件夹
		homeDir, _ := os.UserHomeDir()
		outputDir = filepath.Join(homeDir, "Desktop", "PDF转换结果")
	}

	// 如果有相对路径信息，从第一个文件提取父目录名
	if len(paths) > 0 && paths[0] != "" {
		// 提取顶层文件夹名称
		parts := strings.Split(paths[0], string(filepath.Separator))
		if len(parts) > 0 {
			topFolder := parts[0]
			outputDir = filepath.Join(outputDir, topFolder)
		}
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("创建输出目录失败: %v", err), http.StatusInternalServerError)
		return
	}

	successCount := 0
	failedCount := 0

	// 处理每个上传的PDF文件
	for i, fileHeader := range files {
		if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".pdf") {
			continue
		}

		// 打开上传的文件
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("打开文件失败 %s: %v\n", fileHeader.Filename, err)
			failedCount++
			continue
		}

		// 转换PDF为文本
		text, err := convertPDFReaderToText(file)
		file.Close()

		if err != nil {
			log.Printf("转换失败 %s: %v\n", fileHeader.Filename, err)
			failedCount++
			continue
		}

		// 确定输出文件路径
		var outputPath string
		if i < len(paths) && paths[i] != "" {
			// 使用相对路径结构
			relPath := paths[i]
			// 移除顶层文件夹（已包含在 outputDir 中）
			parts := strings.Split(relPath, string(filepath.Separator))
			if len(parts) > 1 {
				relPath = filepath.Join(parts[1:]...)
			} else {
				relPath = parts[0]
			}
			txtFileName := strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".txt"
			outputPath = filepath.Join(outputDir, txtFileName)
		} else {
			txtFileName := strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename)) + ".txt"
			outputPath = filepath.Join(outputDir, txtFileName)
		}

		// 确保子目录存在
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			log.Printf("创建子目录失败 %s: %v\n", filepath.Dir(outputPath), err)
			failedCount++
			continue
		}

		// 写入文件
		if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
			log.Printf("写入文件失败 %s: %v\n", outputPath, err)
			failedCount++
			continue
		}

		successCount++
		log.Printf("转换成功: %s -> %s\n", fileHeader.Filename, outputPath)
	}

	// 打开输出目录
	if err := openFolder(outputDir); err != nil {
		log.Printf("打开文件夹失败: %v\n", err)
	}

	// 返回结果
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"successCount": successCount,
		"failedCount":  failedCount,
		"outputPath":   outputDir,
	})

	log.Printf("本地保存完成: 成功 %d, 失败 %d, 输出目录: %s\n", successCount, failedCount, outputDir)
}

// openFolder 打开指定文件夹
func openFolder(path string) error {
	var cmd *exec.Cmd

	// 检测操作系统
	output, _ := exec.Command("uname").Output()
	platform := strings.ToLower(string(output))

	if strings.Contains(platform, "darwin") {
		// macOS
		cmd = exec.Command("open", path)
	} else if strings.Contains(platform, "linux") {
		// Linux
		cmd = exec.Command("xdg-open", path)
	} else {
		// Windows
		cmd = exec.Command("cmd", "/c", "start", path)
	}

	return cmd.Start()
}

// convertPDFReaderToText 从io.Reader读取PDF并转换为文本
func convertPDFReaderToText(r io.Reader) (string, error) {
	// 读取所有数据到内存
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("读取PDF数据失败: %w", err)
	}

	// 首先尝试使用unipdf
	text, err := convertWithUnipdf(data)
	if err == nil {
		return text, nil
	}

	// 如果unipdf失败，尝试使用pdftotext
	log.Printf("unipdf转换失败: %v，尝试使用pdftotext", err)
	text, err = convertWithPdftotext(data)
	if err == nil {
		return text, nil
	}

	return "", fmt.Errorf("所有转换方法都失败了: unipdf和pdftotext都不可用")
}

// convertWithUnipdf 使用unipdf库转换PDF
func convertWithUnipdf(data []byte) (string, error) {
	// 创建bytes.Reader以支持Seek
	reader := bytes.NewReader(data)

	// 创建PDF阅读器
	pdfReader, err := pdf.NewPdfReader(reader)
	if err != nil {
		return "", fmt.Errorf("创建PDF阅读器失败: %w", err)
	}

	// 获取页数
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return "", fmt.Errorf("获取页数失败: %w", err)
	}

	// 提取所有页面的文本
	var allText strings.Builder
	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return "", fmt.Errorf("获取第%d页失败: %w", i, err)
		}

		ex, err := extractor.New(page)
		if err != nil {
			return "", fmt.Errorf("创建提取器失败（第%d页）: %w", i, err)
		}

		text, err := ex.ExtractText()
		if err != nil {
			return "", fmt.Errorf("提取文本失败（第%d页）: %w", i, err)
		}

		allText.WriteString(text)
		allText.WriteString("\n")
	}

	return allText.String(), nil
}

// convertWithPdftotext 使用pdftotext命令行工具转换PDF
func convertWithPdftotext(data []byte) (string, error) {
	// 检查pdftotext是否可用
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("pdftotext命令不可用，请安装poppler-utils")
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "pdf2txt-*.pdf")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// 写入PDF数据
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	// 执行pdftotext命令
	cmd := exec.Command("pdftotext", "-layout", tmpPath, "-")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext执行失败: %w", err)
	}

	return string(output), nil
}

// convertPDFToText 将单个PDF文件转换为文本文件
func convertPDFToText(pdfPath string, outputDir string) error {
	// 读取PDF文件
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("读取PDF文件失败: %w", err)
	}

	// 首先尝试使用unipdf
	text, err := convertWithUnipdf(data)
	if err != nil {
		// 如果unipdf失败，尝试使用pdftotext
		log.Printf("unipdf转换失败: %v，尝试使用pdftotext", err)
		text, err = convertWithPdftotext(data)
		if err != nil {
			return fmt.Errorf("所有转换方法都失败了: %w", err)
		}
	}

	// 生成输出文件名
	baseName := filepath.Base(pdfPath)
	txtFileName := strings.TrimSuffix(baseName, filepath.Ext(baseName)) + ".txt"
	outputPath := filepath.Join(outputDir, txtFileName)

	// 写入文件
	err = os.WriteFile(outputPath, []byte(text), 0644)
	if err != nil {
		return fmt.Errorf("写入TXT文件失败: %w", err)
	}

	return nil
}

const htmlTemplate = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PDF转TXT批量转换工具</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            max-width: 1000px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        .header h1 {
            font-size: 28px;
            margin-bottom: 10px;
        }
        .header p {
            opacity: 0.9;
            font-size: 14px;
        }
        .main {
            padding: 30px;
        }
        .section {
            margin-bottom: 30px;
        }
        .section-title {
            font-size: 18px;
            font-weight: 600;
            margin-bottom: 15px;
            color: #333;
        }
        .path-display {
            background: #f5f5f5;
            padding: 12px 15px;
            border-radius: 6px;
            font-family: monospace;
            font-size: 14px;
            margin-bottom: 15px;
            word-break: break-all;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .path-display .path {
            flex: 1;
        }
        .path-display button {
            background: #667eea;
            color: white;
            border: none;
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            white-space: nowrap;
        }
        .path-display button:hover {
            background: #5568d3;
        }
        .file-browser {
            border: 1px solid #e0e0e0;
            border-radius: 6px;
            max-height: 400px;
            overflow-y: auto;
        }
        .file-item {
            padding: 12px 15px;
            border-bottom: 1px solid #f0f0f0;
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 10px;
            transition: background 0.2s;
        }
        .file-item:hover {
            background: #f9f9f9;
        }
        .file-item:last-child {
            border-bottom: none;
        }
        .file-item .icon {
            font-size: 20px;
        }
        .file-item .name {
            flex: 1;
            font-size: 14px;
        }
        .dir-item {
            color: #667eea;
            font-weight: 500;
        }
        .input-group {
            margin-bottom: 15px;
        }
        .input-group label {
            display: block;
            margin-bottom: 8px;
            font-size: 14px;
            font-weight: 500;
            color: #555;
        }
        .input-group input {
            width: 100%;
            padding: 12px 15px;
            border: 1px solid #e0e0e0;
            border-radius: 6px;
            font-size: 14px;
            font-family: monospace;
        }
        .input-group input:focus {
            outline: none;
            border-color: #667eea;
        }
        .btn {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            padding: 14px 30px;
            border-radius: 6px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            width: 100%;
            transition: transform 0.2s;
        }
        .btn:hover {
            transform: translateY(-2px);
        }
        .btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
        }
        .results {
            margin-top: 30px;
            padding: 20px;
            background: #f9f9f9;
            border-radius: 6px;
            display: none;
        }
        .results.show {
            display: block;
        }
        .results h3 {
            margin-bottom: 15px;
            color: #333;
        }
        .results .success-list, .results .failed-list {
            margin-bottom: 20px;
        }
        .results .success-list h4 {
            color: #10b981;
            margin-bottom: 10px;
        }
        .results .failed-list h4 {
            color: #ef4444;
            margin-bottom: 10px;
        }
        .results ul {
            list-style: none;
            font-size: 13px;
            font-family: monospace;
            max-height: 200px;
            overflow-y: auto;
        }
        .results ul li {
            padding: 5px 0;
            border-bottom: 1px solid #e0e0e0;
        }
        .loading {
            display: none;
            text-align: center;
            padding: 20px;
            color: #667eea;
        }
        .loading.show {
            display: block;
        }
        .spinner {
            border: 3px solid #f3f3f3;
            border-top: 3px solid #667eea;
            border-radius: 50%;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 0 auto 15px;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>PDF转TXT批量转换工具</h1>
            <p>选择包含PDF文件的目录，一键批量转换为文本文件</p>
        </div>

        <div class="main">
            <div class="section">
                <div class="section-title">1. 选择PDF文件</div>
                <input type="file" id="folderInput" webkitdirectory directory multiple style="display: none;" onchange="handleFolderSelect()">
                <button class="btn" onclick="document.getElementById('folderInput').click()" style="margin-bottom: 20px;">
                    📁 从系统选择文件夹
                </button>
                <div id="fileInfo" style="display: none; padding: 15px; background: #f0f7ff; border-radius: 6px; margin-bottom: 20px;">
                    <div style="font-weight: 600; margin-bottom: 8px;">已选择 <span id="pdfCount">0</span> 个PDF文件</div>
                    <div style="font-size: 13px; color: #666; max-height: 150px; overflow-y: auto;" id="fileList"></div>
                </div>
            </div>

            <div class="section" id="outputSection" style="display: none;">
                <div class="section-title">2. 选择输出方式</div>
                <div style="margin-bottom: 20px;">
                    <label style="display: block; margin-bottom: 10px; cursor: pointer;">
                        <input type="radio" name="outputMode" value="download" checked onchange="toggleOutputMode()">
                        <span style="margin-left: 8px;">下载为ZIP文件（保存到浏览器下载文件夹）</span>
                    </label>
                    <label style="display: block; cursor: pointer;">
                        <input type="radio" name="outputMode" value="local" onchange="toggleOutputMode()">
                        <span style="margin-left: 8px;">保存到本地文件夹并自动打开</span>
                    </label>
                </div>
                <div id="localOutputOptions" style="display: none;">
                    <div class="input-group">
                        <label>选择输出文件夹（留空则保存在桌面的 PDF转换结果 文件夹）</label>
                        <input type="text" id="localOutputDir" placeholder="留空使用默认位置：~/Desktop/PDF转换结果">
                    </div>
                </div>
            </div>

            <div class="section">
                <button class="btn" onclick="startProcess()" id="processBtn" style="display: none;">
                    开始转换
                </button>
            </div>

            <div class="loading" id="loading">
                <div class="spinner"></div>
                <div>正在转换中，请稍候...</div>
            </div>

            <div class="results" id="results">
                <h3>转换结果</h3>
                <div class="success-list">
                    <h4>成功 (<span id="successCount">0</span>)</h4>
                    <ul id="successList"></ul>
                </div>
                <div class="failed-list">
                    <h4>失败 (<span id="failedCount">0</span>)</h4>
                    <ul id="failedList"></ul>
                </div>
            </div>
        </div>
    </div>

    <script>
        let selectedFiles = [];

        function toggleOutputMode() {
            const mode = document.querySelector('input[name="outputMode"]:checked').value;
            const localOptions = document.getElementById('localOutputOptions');
            localOptions.style.display = mode === 'local' ? 'block' : 'none';
        }

        function handleFolderSelect() {
            const input = document.getElementById('folderInput');
            const files = Array.from(input.files);
            selectedFiles = files.filter(file => file.name.toLowerCase().endsWith('.pdf'));

            if (selectedFiles.length === 0) {
                alert('所选文件夹中没有找到PDF文件');
                return;
            }

            document.getElementById('pdfCount').textContent = selectedFiles.length;
            const fileList = document.getElementById('fileList');
            fileList.innerHTML = '';
            selectedFiles.forEach(file => {
                const div = document.createElement('div');
                div.textContent = file.webkitRelativePath || file.name;
                div.style.padding = '3px 0';
                fileList.appendChild(div);
            });

            document.getElementById('fileInfo').style.display = 'block';
            document.getElementById('outputSection').style.display = 'block';
            document.getElementById('processBtn').style.display = 'block';
        }

        function startProcess() {
            const mode = document.querySelector('input[name="outputMode"]:checked').value;
            if (mode === 'download') {
                uploadAndConvert();
            } else {
                uploadAndSaveLocal();
            }
        }

        async function uploadAndConvert() {
            if (selectedFiles.length === 0) {
                alert('请先选择包含PDF文件的文件夹');
                return;
            }

            const formData = new FormData();
            selectedFiles.forEach(file => {
                formData.append('files', file);
            });

            document.getElementById('processBtn').disabled = true;
            document.getElementById('loading').classList.add('show');
            document.getElementById('results').classList.remove('show');

            try {
                const response = await fetch('/api/upload-convert', {
                    method: 'POST',
                    body: formData
                });

                if (!response.ok) {
                    throw new Error('转换失败: ' + response.statusText);
                }

                const blob = await response.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'converted-texts.zip';
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
                document.body.removeChild(a);

                alert('转换完成！ZIP文件已保存到浏览器下载文件夹。\n\n提示：通常位于 ~/Downloads/ 目录');
            } catch (error) {
                alert('转换失败: ' + error.message);
            } finally {
                document.getElementById('processBtn').disabled = false;
                document.getElementById('loading').classList.remove('show');
            }
        }

        async function uploadAndSaveLocal() {
            if (selectedFiles.length === 0) {
                alert('请先选择包含PDF文件的文件夹');
                return;
            }

            const formData = new FormData();
            selectedFiles.forEach(file => {
                formData.append('files', file);
                // 发送文件的相对路径，用于在服务器端还原目录结构
                formData.append('paths', file.webkitRelativePath || file.name);
            });

            const outputDir = document.getElementById('localOutputDir').value;
            if (outputDir) {
                formData.append('outputDir', outputDir);
            }

            document.getElementById('processBtn').disabled = true;
            document.getElementById('loading').classList.add('show');
            document.getElementById('results').classList.remove('show');

            try {
                const response = await fetch('/api/upload-save-local', {
                    method: 'POST',
                    body: formData
                });

                const result = await response.json();

                if (!response.ok) {
                    throw new Error(result.error || '转换失败');
                }

                alert('转换完成！\n\n' +
                      '成功: ' + result.successCount + ' 个文件\n' +
                      '失败: ' + result.failedCount + ' 个文件\n\n' +
                      '文件已保存到: ' + result.outputPath + '\n\n' +
                      '文件夹将自动打开...');

            } catch (error) {
                alert('转换失败: ' + error.message);
            } finally {
                document.getElementById('processBtn').disabled = false;
                document.getElementById('loading').classList.remove('show');
            }
        }

    </script>
</body>
</html>
`
