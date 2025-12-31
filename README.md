# PDF转TXT批量转换工具

一个简单高效的PDF文件批量转换工具，可以将整个目录下的所有PDF文件转换为TXT文本文件。

## 功能特点

- 批量转换整个目录下的所有PDF文件
- 自动遍历子目录
- 支持自定义输出目录
- 提供Web界面和命令行两种使用方式
- Web界面支持可视化目录浏览和选择

## 安装

```bash
cd pdf2txt
go build -o pdf2txt main.go
```

## 使用方法

### 方式一：Web界面模式（推荐）

启动Web服务器，通过浏览器进行可视化操作：

```bash
./pdf2txt -web
```

然后在浏览器中打开 http://localhost:8082（默认端口已改为8082）

指定自定义端口：

```bash
./pdf2txt -web -port 9000
```

Web界面功能：
- **通过系统文件选择对话框直接选择文件夹**
- 自动识别文件夹中的所有PDF文件
- 两种输出方式：
  - 下载ZIP压缩包（保存到浏览器下载文件夹）
  - 保存到本地文件夹并自动打开（推荐）

### 方式二：命令行模式

将PDF文件转换到同一目录：

```bash
./pdf2txt -input /path/to/pdf/directory
```

指定输出目录：

```bash
./pdf2txt -input /path/to/pdf/directory -output /path/to/output/directory
```

## 参数说明

- `-web`: 启动Web服务器模式
- `-port`: Web服务器端口（默认8082）
- `-input`: （必需，仅命令行模式）包含PDF文件的输入目录
- `-output`: （可选，仅命令行模式）输出TXT文件的目录，默认与输入目录相同

## Web界面使用说明

### 步骤 1：选择 PDF 文件
- 点击"📁 从系统选择文件夹"按钮
- 在弹出的系统对话框中选择包含 PDF 文件的文件夹
- 系统会自动识别所有 PDF 文件并显示列表

### 步骤 2：选择输出方式

**方式一：下载为 ZIP 文件（适合小批量文件）**
- 选择"下载为ZIP文件"选项
- 点击"开始转换"
- 转换完成后会自动下载 `converted-texts.zip`
- ZIP 文件保存在浏览器的下载文件夹（通常是 ~/Downloads/）

**方式二：保存到本地文件夹（推荐，适合大批量文件）**
- 选择"保存到本地文件夹并自动打开"选项
- 可选：指定输出文件夹（留空则使用默认位置）
- 默认位置：`~/Desktop/PDF转换结果/[原文件夹名称]/`
- 点击"开始转换"
- 转换完成后会自动打开输出文件夹
- **优点**：文件直接保存到指定位置，转换完成立即可见

## 示例

### Web界面模式

```bash
# 使用默认端口8082启动Web界面
./pdf2txt -web

# 使用自定义端口9000启动Web界面
./pdf2txt -web -port 9000
```

然后在浏览器中：
1. 点击"📁 从系统选择文件夹"按钮
2. 在弹出的系统对话框中选择包含 PDF 文件的文件夹
3. 确认后会显示找到的 PDF 文件列表
4. 选择输出方式：
   - **下载 ZIP**：适合小批量文件，下载后自己解压
   - **保存到本地**（推荐）：文件直接保存到桌面，自动打开文件夹
5. 点击"开始转换"
6. 稍等片刻，转换完成！

### 命令行模式

```bash
# 转换当前目录下的所有PDF文件
./pdf2txt -input .

# 转换指定目录的PDF文件，并输出到另一个目录
./pdf2txt -input ~/Documents/pdfs -output ~/Documents/texts

# 转换外置硬盘上的PDF文件
./pdf2txt -input "/Volumes/G盘/18-人人都用得上的写作课"
```

## 注意事项

### Web界面模式
1. **推荐使用"保存到本地文件夹"模式**
   - 文件直接保存到指定位置，无需解压
   - 转换完成后自动打开文件夹，立即可见结果
   - 默认保存位置：`~/Desktop/PDF转换结果/[原文件夹名称]/`

2. **关于文件选择**
   - 浏览器会打开系统原生的文件夹选择对话框
   - 支持选择任意位置的文件夹
   - 会自动识别所有 PDF 文件（包括子文件夹）

3. **转换时间**
   - 大文件或大量文件可能需要较长时间，请耐心等待
   - 转换过程中会显示加载动画

4. **输出位置**
   - 下载 ZIP 模式：文件保存在浏览器下载文件夹（通常是 ~/Downloads/）
   - 本地文件夹模式：文件保存在指定位置（默认桌面），并自动打开

### 命令行模式
1. 程序会自动创建输出目录（如果不存在）
2. 转换后的TXT文件名与原PDF文件名相同，仅扩展名改为.txt
3. 如果转换某个文件失败，程序会显示错误信息并继续处理其他文件
4. 程序会递归处理所有子目录中的PDF文件

## 依赖

### 必需依赖
- [unipdf](https://github.com/lu4p/unipdf) - PDF处理库（已内置）

### 可选依赖（推荐安装）
- **pdftotext** (来自 poppler-utils) - 用于处理某些特殊格式的PDF文件

当 unipdf 无法处理某些 PDF 文件时（如使用 PostScript 字体的 PDF），程序会自动尝试使用 pdftotext 作为备用方案。

安装 pdftotext：

**macOS (使用 Homebrew):**
```bash
brew install poppler
```

**Ubuntu/Debian:**
```bash
sudo apt-get install poppler-utils
```

**CentOS/RHEL:**
```bash
sudo yum install poppler-utils
```

## 故障排除

### 转换失败：fonts based on PostScript outlines are not supported

这个错误表示 PDF 使用了 unipdf 不支持的 PostScript 字体。解决方案：

1. **安装 pdftotext**（推荐）：按照上面的依赖说明安装 poppler-utils
2. 安装后重新运行转换，程序会自动使用 pdftotext 作为备用方案
3. 查看日志输出，确认是否成功切换到 pdftotext

## 许可

本工具仅供个人学习和使用。
