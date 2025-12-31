# PDF转TXT批量转换工具

一个简单高效的PDF文件批量转换工具，可以将整个目录下的所有PDF文件转换为TXT文本文件。

## 功能特点

- 批量转换整个目录下的所有PDF文件
- 自动遍历子目录
- 支持自定义输出目录

## 安装

```bash
cd pdf2txt
go build -o pdf2txt main.go
```

## 使用方法

### 基本用法

将PDF文件转换到同一目录：

```bash
./pdf2txt -input /path/to/pdf/directory
```

### 指定输出目录

将PDF文件转换到指定目录：

```bash
./pdf2txt -input /path/to/pdf/directory -output /path/to/output/directory
```

## 参数说明

- `-input`: （必需）包含PDF文件的输入目录
- `-output`: （可选）输出TXT文件的目录，默认与输入目录相同

## 示例

```bash
# 转换当前目录下的所有PDF文件
./pdf2txt -input .

# 转换指定目录的PDF文件，并输出到另一个目录
./pdf2txt -input ~/Documents/pdfs -output ~/Documents/texts

# 转换外置硬盘上的PDF文件
./pdf2txt -input "/Volumes/G盘/18-人人都用得上的写作课"
```

## 注意事项

1. 程序会自动创建输出目录（如果不存在）
2. 转换后的TXT文件名与原PDF文件名相同，仅扩展名改为.txt
3. 如果转换某个文件失败，程序会显示错误信息并继续处理其他文件
4. 程序会递归处理所有子目录中的PDF文件

## 依赖

- [unipdf](https://github.com/lu4p/unipdf) - PDF处理库

## 许可

本工具仅供个人学习和使用。
