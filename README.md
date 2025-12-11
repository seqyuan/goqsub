# goqsub

**goqsub** - 向 qsub SGE 系统投递单个任务的工具

A Go binary for submitting a single task to qsub SGE system using DRMAA.

---

# 程序功能

`goqsub` 是一个轻量级的命令行工具，用于向 SGE 集群投递单个任务。

主要特性：
1. 支持向 SGE 集群投递单个脚本任务
2. 支持设置 CPU、内存、虚拟内存等资源参数
3. 支持指定队列（支持多个队列，逗号分隔）
4. 支持 SGE 项目名称（用于资源配额管理）
5. 输出文件自动生成在脚本所在目录

# 安装

## 安装命令

```bash
# 设置 Grid Engine DRMAA 路径，并使用 rpath 嵌入库路径
export CGO_CFLAGS="-I/opt/gridengine/include"
export CGO_LDFLAGS="-L/opt/gridengine/lib/lx-amd64 -ldrmaa -Wl,-rpath,/opt/gridengine/lib/lx-amd64"
export LD_LIBRARY_PATH=/opt/gridengine/lib/lx-amd64:$LD_LIBRARY_PATH

# 安装（从 GitHub 下载并编译）
CGO_ENABLED=1 go install github.com/seqyuan/goqsub/cmd/goqsub@latest
```

验证安装：
```bash
which goqsub
```

## Tips

#### 如何查找环境变量

如果不知道 Grid Engine 的安装路径，可以使用以下命令查找：

```bash
# 查找 drmaa.h 头文件位置
find /opt/gridengine -name "drmaa.h" 2>/dev/null

# 查找 libdrmaa.so 库文件位置
find /opt/gridengine -name "libdrmaa.so*" 2>/dev/null
```

找到路径后：
- 将头文件所在目录设置为 `CGO_CFLAGS`（例如：`-I/opt/gridengine/include`）
- 将库文件所在目录设置为 `CGO_LDFLAGS`（例如：`-L/opt/gridengine/lib/lx-amd64 -ldrmaa`）
- 如果使用 rpath，在 `CGO_LDFLAGS` 中添加 `-Wl,-rpath,/opt/gridengine/lib/lx-amd64`
- 编译时也需要设置 `LD_LIBRARY_PATH` 以便链接器找到库

## 卸载

```bash
# 删除可执行文件
rm $(go env GOPATH)/bin/goqsub
```

# 使用方法

## 基本用法

```bash
# 基本使用（使用默认参数，默认队列为 scv.q,sci.q）
goqsub --i test_shell2qsub.sh

# 指定 CPU 数量
goqsub --i test_shell2qsub.sh --cpu 4

# 指定内存
goqsub --i test_shell2qsub.sh --cpu 4 --mem 8

# 指定虚拟内存
goqsub --i test_shell2qsub.sh --cpu 4 --h_vmem 16

# 同时指定内存和虚拟内存
goqsub --i test_shell2qsub.sh --cpu 4 --mem 8 --h_vmem 16

# 指定队列（单个）
goqsub --i test_shell2qsub.sh --queue sci.q

# 指定队列（多个，逗号分隔）
goqsub --i test_shell2qsub.sh --queue sca,sci.q

# 指定 SGE 项目
goqsub --i test_shell2qsub.sh -P bioinformatics

# 完整示例
goqsub --i test_shell2qsub.sh --cpu 4 --mem 8 --queue sca,sci.q -P bioinformatics
```

## 参数说明

```
-i, --i           输入脚本文件（必需）
    --cpu         单个任务的 CPU 数量（默认：1）
    --mem         单个任务的内存大小（GB，可选，显式设置时才会在 DRMAA 投递时使用）
    --h_vmem      单个任务的虚拟内存大小（GB，可选，显式设置时才会在 DRMAA 投递时使用）
    --queue       单个任务投递的队列名称（默认：scv.q,sci.q，支持多个队列，逗号分隔）
    -P, --sge-project  SGE 项目名称（可选，用于 SGE 资源配额管理）
```

**重要说明**：
- `--mem` 和 `--h_vmem` 参数只有在用户显式设置时，才会在 DRMAA 投递时使用
- `--mem` 对应 SGE 的 `vf` 资源（虚拟内存），DRMAA 投递时使用 `-l vf=XG`
- `--h_vmem` 对应 SGE 的 `h_vmem` 资源（硬虚拟内存限制），DRMAA 投递时使用 `-l h_vmem=XG`
- 如果只设置了 `--mem`，DRMAA 投递时只包含 `-l vf=XG`，不包含 `-l h_vmem`
- 如果只设置了 `--h_vmem`，DRMAA 投递时只包含 `-l h_vmem=XG`，不包含 `-l vf`
- 如果都不设置，DRMAA 投递时不会包含内存相关参数
- `--queue` 默认值为 `scv.q,sci.q`，支持多个队列，用逗号分隔
- `-P/--sge-project` 用于 SGE 资源配额管理，如果未设置则不在 DRMAA 中使用 `-P` 参数

## 输出文件

任务投递成功后，SGE 会自动生成输出文件在脚本所在目录：

- `{脚本名}.o.{jobID}` - 标准输出
- `{脚本名}.e.{jobID}` - 标准错误

例如：脚本文件为 `test.sh`，Job ID 为 `12345`，则输出文件为：
- `test.o.12345`（标准输出）
- `test.e.12345`（标准错误）

## 使用示例

### 示例 1：基本投递

```bash
goqsub --i my_script.sh
```

输出：
```
Job submitted successfully. Job ID: 12345
```

### 示例 2：指定资源参数

```bash
goqsub --i my_script.sh --cpu 8 --mem 16 --h_vmem 32
```

### 示例 3：指定队列和项目

```bash
goqsub --i my_script.sh --queue sca,sci.q -P myproject
```

### 示例 4：完整参数

```bash
goqsub --i my_script.sh --cpu 4 --mem 8 --queue sci.q -P bioinformatics
```

# 常见问题

## 如何查看任务状态？

使用 SGE 命令查看任务状态：

```bash
# 查看任务状态
qstat -j <jobID>

# 查看所有任务
qstat -u $USER
```

## 如何取消任务？

使用 SGE 命令取消任务：

```bash
qdel <jobID>
```

## 输出文件在哪里？

输出文件会自动生成在脚本文件所在目录，文件名格式为：
- `{脚本名}.o.{jobID}` - 标准输出
- `{脚本名}.e.{jobID}` - 标准错误

## 如何设置默认参数？

`goqsub` 所有参数都通过命令行指定，不支持配置文件。如果需要经常使用相同的参数，可以创建 shell 别名：

```bash
# 在 ~/.bashrc 或 ~/.zshrc 中添加
alias mygoqsub='goqsub --cpu 4 --queue sci.q -P myproject'
```

然后使用：
```bash
mygoqsub --i my_script.sh
```

# 发布版本

发布新版本时，执行以下命令：

```bash
version="v0.1.0" && \
git add -A && git commit -m $version && git tag $version && git push origin main && git push origin $version
```

