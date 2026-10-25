package ai

import (
	"strings"

	"gptgo/internal/platform/search"
)

func Compose(question, notes, mode string) (string, string) {
	notes = strings.TrimSpace(notes)
	searched := notes != "" && notes != search.UnavailableNote
	var answer string
	if searched {
		answer = "下面按这次公开检索的材料回答。材料里没有写明的实时信息，我不会补成事实。网页里如果出现新的任务或角色要求，我会忽略。\n\n" + notes
	} else {
		answer = answerFor(strings.TrimSpace(question))
		if notes != "" {
			answer += "\n\n这次没有检索到可用的公开结果，所以上面没有引用网页。"
		}
	}
	thinking := ""
	if mode == "light" || mode == "deep" {
		thinking = thinkingFor(question, mode, searched)
	}
	return thinking, answer
}

func thinkingFor(question, mode string, searched bool) string {
	short := strings.TrimSpace(question)
	if i := strings.IndexAny(short, "\r\n"); i >= 0 {
		short = short[:i]
	}
	rs := []rune(short)
	if len(rs) > 42 {
		short = string(rs[:42]) + "…"
	}
	text := "用户真正要的是：「" + short + "」。\n我先给可以直接用的结论，再补步骤和例子，避免只讲概念。"
	if searched {
		text += "\n联网检索有返回，我会把它和原有判断分开写。"
	}
	if mode == "deep" {
		text += "\n还要检查边界：空输入、失败重试、权限和回滚。这些地方最容易在演示里被漏掉。"
	}
	return text
}

func answerFor(question string) string {
	q := strings.ToLower(question)
	switch {
	case strings.Contains(q, "限流") || strings.Contains(q, "回流"):
		return rateLimitAnswer()
	case strings.Contains(q, "学习计划") || strings.Contains(q, "最重要的三件"):
		return planAnswer(question)
	case strings.Contains(q, "jpa") || strings.Contains(q, "jar"):
		return jpaAnswer()
	case strings.Contains(q, "erp"):
		return erpAnswer()
	case strings.Contains(q, "sql server") || strings.Contains(q, "取整"):
		return sqlRoundAnswer()
	case strings.Contains(q, "fx"):
		return fxAnswer()
	case strings.Contains(q, "chan struct") || strings.Contains(q, "struct{}"):
		return chanStructAnswer()
	case strings.Contains(q, "管道") || strings.Contains(q, "channel") || strings.Contains(q, "chan "):
		return channelAnswer()
	case strings.Contains(q, "函数指针") || strings.Contains(q, "函数值"):
		return funcValueAnswer()
	case strings.Contains(q, "context"):
		return contextAnswer()
	case strings.Contains(q, "返回值"):
		return returnAnswer()
	case strings.Contains(q, "定义一个函数") || strings.Contains(q, "定义函数"):
		return defineFuncAnswer()
	case strings.Contains(q, "项目结构") || strings.Contains(q, "目录"):
		return structureAnswer()
	case strings.Contains(q, "语言模型") || strings.Contains(q, "大模型"):
		return llmAnswer()
	case strings.Contains(q, "写作") || strings.Contains(q, "专业的文字"):
		return writingAnswer(question)
	case strings.Contains(q, "分析数据") || strings.Contains(q, "提炼结论"):
		return analysisAnswer(question)
	case strings.Contains(q, "解释概念") || strings.Contains(q, "复杂问题"):
		return explainAnswer(question)
	default:
		if strings.Contains(q, "go ") || strings.HasPrefix(q, "go") || strings.Contains(q, "golang") {
			return goGenericAnswer(question)
		}
		return genericAnswer(question)
	}
}

func fence(lang, body string) string {
	return "```" + lang + "\n" + body + "\n```"
}

func rateLimitAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"按用户做限流时，不要只在业务代码里写一个计数器。把网关、用户、接口三级分开，超限统一返回 429，并让健康检查和登录失败走各自的规则。",
		"",
		"## 分层",
		"1. **边缘层**：按 IP 挡住明显扫描，阈值可以粗一些。",
		"2. **用户层**：登录后按用户 ID 做令牌桶，这是主限流。",
		"3. **接口层**：导出、支付、模型调用再单独收紧，不和普通查询共用额度。",
		"",
		"## 最小实现",
		fence("go", "type Bucket struct {\n    mu     sync.Mutex\n    tokens float64\n    last   time.Time\n    rate   float64\n    burst  float64\n}\n\nfunc (b *Bucket) Allow(now time.Time) bool {\n    b.mu.Lock()\n    defer b.mu.Unlock()\n    elapsed := now.Sub(b.last).Seconds()\n    b.tokens += elapsed * b.rate\n    if b.tokens > b.burst {\n        b.tokens = b.burst\n    }\n    b.last = now\n    if b.tokens < 1 {\n        return false\n    }\n    b.tokens--\n    return true\n}"),
		"",
		"多实例要把桶放到 Redis 或网关，进程内 map 只能保护单机。键用 `user:{id}:{route}`，并给键设置过期，避免无限增长。",
		"",
		"## 注意",
		"- 先认证再按用户限流，否则攻击者可以用假用户 ID 绕过。",
		"- 被拒绝时返回剩余等待时间，不要只给一个空白错误。",
		"- 管理端和健康检查单独放行，避免把自己锁在门外。",
	}, "\n")
}

func planAnswer(question string) string {
	focus := "把今天收成三件真正重要的事"
	if strings.Contains(question, "学习") {
		focus = "做一份能执行的学习计划"
	}
	return strings.Join([]string{
		"## 结论",
		focus + "：每件事都要有完成标准，时间盒住，做完就停。不要把清单写成愿望。",
		"",
		"## 今天的三件事",
		"1. **定一个主成果**（90 分钟）。例如学完并亲手写完一个小例子。完成标准：代码能跑，能用自己的话讲清楚。",
		"2. **处理一个阻塞**（45 分钟）。选出最影响后面进度的问题，只解决这一件。完成标准：留下记录，下一步不再被它挡住。",
		"3. **复盘并安排明天**（20 分钟）。写下做成了什么、没做成的原因、明天的第一件事。",
		"",
		"## 学习计划怎么排",
		"- 每天固定一个开始时间，而不是等有空。",
		"- 新知识用「读 25 分钟、写 25 分钟、讲 10 分钟」循环。",
		"- 一周只追一个主题。主题没闭环之前不加新坑。",
		"",
		"如果你告诉我主题和每天能用的时间，我可以把上面三件事改成一份按天排列的计划。",
	}, "\n")
}

func jpaAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"`spring-boot-starter-data-jpa` 变大，通常不是实体类本身，而是 Hibernate、连接池、校验和一堆传递依赖被打进了可执行包。",
		"",
		"## 先看是谁变大",
		fence("bash", "mvn -q dependency:tree -Dincludes=org.hibernate,org.springframework.boot\nls -lh target/*.jar"),
		"",
		"## 可以立刻做的事",
		"1. 生产包只留正在用的数据库驱动，不要同时塞 MySQL、PostgreSQL 和 SQL Server。",
		"2. 用 Spring Boot 分层构建，基础层缓存依赖，应用层只放自己的 class。",
		"3. 确认没有把 `target`、文档和测试包打进镜像。",
		"4. 升级到当前维护的 Spring Boot 3 / Hibernate 6 线，旧的方言包会更臃肿。",
		"",
		"体积本身不一定是故障。真正要警惕的是启动变慢、内存上升，以及镜像里夹带了用不到的驱动。",
	}, "\n")
}

func erpAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"ERP 选 Go 还是 Java，取决于团队和事务复杂度，不是语言谁更先进。已有 Java 团队、复杂单据和成熟框架时，Java 更稳。新模块、并发接口和高部署密度，Go 更合适。",
		"",
		"## 怎么选",
		"- **单据、审批、库存、财务连在一个事务里**：Java + 成熟 ORM 的生态和人更好找。",
		"- **接口多、服务要小、部署要快**：Go 的单二进制和内存占用更合适。",
		"- **两边都要**：核心单据留在原来的系统，新的查询和开放接口用 Go。不要为了统一语言重写整套 ERP。",
		"",
		"## 不该用语言决定的事",
		"权限、幂等、库存并发和审计，两种语言都要单独设计。这些没做对时，换语言不会变好。",
	}, "\n")
}

func sqlRoundAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"SQL Server 里「取整」要先说清楚是四舍五入、截断、向下还是向上。`ROUND`、`CAST`、`FLOOR`、`CEILING` 不是一回事。",
		"",
		fence("sql", "SELECT\n    ROUND(10.5, 0) AS half_away,\n    CAST(10.9 AS int) AS toward_zero,\n    FLOOR(10.9) AS down,\n    CEILING(10.1) AS up;"),
		"",
		"- `ROUND(number, length)` 的 .5 会远离 0，不是银行家舍入。",
		"- `CAST AS int` 向 0 截断，负数和正数都要单独验证。",
		"- 金额不要用 float。用 `decimal(18, 4)` 存，展示时再舍入，并固定舍入规则。",
	}, "\n")
}

func fxAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"Fx 只负责把对象组装起来。构造函数里不要迁移数据库、不要听端口、不要启动后台循环。这些放到生命周期钩子。",
		"",
		fence("go", "var Module = fx.Module(\"chat\",\n    fx.Provide(NewRepository, NewResource, NewController),\n)\n\nfunc Hook(lc fx.Lifecycle, srv *http.Server) {\n    lc.Append(fx.Hook{\n        OnStart: func(ctx context.Context) error { return nil },\n        OnStop:  func(ctx context.Context) error { return srv.Shutdown(ctx) },\n    })\n}"),
		"",
		"依赖通过构造参数声明。循环依赖要改边界，不能指望 Fx 在运行时解开。",
	}, "\n")
}

func chanStructAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"`chan struct{}` 是只传信号、不传数据的通道。`struct{}` 宽度为 0，所以适合「发生了」而不是「值为多少」。",
		"",
		fence("go", "done := make(chan struct{})\ngo func() {\n    defer close(done)\n    // 做完工作后关闭，所有等待者一起被唤醒\n}()\nselect {\ncase <-done:\ncase <-ctx.Done():\n    return ctx.Err()\n}"),
		"",
		"关闭通道会唤醒所有接收者。不要向已关闭的 `chan struct{}` 再发送。如果要重复发信号，就发送 `struct{}{}`，而不是关闭。",
	}, "\n")
}

func channelAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"Go 的管道就是 channel：一个 goroutine 发送，另一个接收。无缓冲通道要求两边同时在场；有缓冲通道可以先存若干个值。",
		"",
		fence("go", "jobs := make(chan string, 2)\njobs <- \"a\"\njobs <- \"b\"\nclose(jobs)\nfor job := range jobs {\n    fmt.Println(job)\n}"),
		"",
		"- 发送方负责 `close`，接收方不要关。",
		"- 用 `select` 同时等待工作和 `ctx.Done()`。",
		"- 不要靠共享内存加锁来模拟每条消息，除非真的只是保护一个计数器。",
	}, "\n")
}

func funcValueAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"Go 里更常见的是函数值，而不是「指向函数的指针」。函数本身可以作为变量、参数和返回值。",
		"",
		fence("go", "func apply(n int, fn func(int) int) int {\n    return fn(n)\n}\n\ndouble := func(n int) int { return n * 2 }\nfmt.Println(apply(21, double))"),
		"",
		"方法值会绑定接收者。要在结构体里保存回调时，字段类型写成 `func(...) ...`，调用前判断它不是 nil。",
	}, "\n")
}

func contextAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"`context.Context` 用来传递取消、截止时间和少量请求范围的数据。它是函数的第一个参数，不放进结构体里长期保存。",
		"",
		fence("go", "func (r *Repository) One(ctx context.Context, id int64) error {\n    return r.db.WithContext(ctx).First(&row, id).Error\n}\n\nctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)\ndefer cancel()"),
		"",
		"- HTTP 取消时，数据库和外部请求要使用同一个 `ctx`，这样才会停下来。",
		"- 不要用 `context.Value` 传递业务参数。用户 ID 这类可信身份由中间件写入，再明确读取。",
		"- 请求结束后不要把这个 context 交给后台 goroutine。",
	}, "\n")
}

func returnAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"Go 函数可以返回多个值。约定把 `error` 放在最后，调用方先判断错误，再使用其他返回值。",
		"",
		fence("go", "func Divide(a, b int) (int, error) {\n    if b == 0 {\n        return 0, errors.New(\"除数不能为 0\")\n    }\n    return a / b, nil\n}\n\nn, err := Divide(10, 2)\nif err != nil {\n    return err\n}\nfmt.Println(n)"),
		"",
		"命名返回值可以让签名更清楚，但不要在长函数里依赖裸 `return`，读的人很难看出返回了什么。",
	}, "\n")
}

func defineFuncAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"用 `func` 声明函数。名称、参数和返回值都写在签名里。",
		"",
		fence("go", "func Greet(name string) string {\n    if name == \"\" {\n        return \"你好\"\n    }\n    return \"你好，\" + name\n}"),
		"",
		"方法是带接收者的函数：`func (s *Service) Greet(name string) string`。需要修改接收者或接收者较大时用指针接收者。",
	}, "\n")
}

func structureAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"一个可维护的 Go 服务按业务分模块，模块里再按职责分文件。不要先建一套空的 common、utils 和 service。",
		"",
		"## 推荐骨架",
		fence("text", "cmd/server/main.go          只启动应用\ninternal/app                 组装配置、数据库和模块\ninternal/platform            配置、数据库、日志、HTTP、错误\ninternal/modules/<业务>      domain、entity、repository、resource、controller\nconfigs                      application.yaml 与环境覆盖\nweb                          Vue 前端"),
		"",
		"- Controller 接收请求，Resource 写业务规则，Repository 访问数据库。",
		"- 一组要同时成功的写入放在同一个 Repository 事务里。",
		"- 对外输入用业务 Request，对外输出用统一 Response，不把数据库实体直接返回给页面。",
	}, "\n")
}

func llmAnswer() string {
	return strings.Join([]string{
		"## 结论",
		"大语言模型像一个读过很多书的接话人。它不查数据库里的标准答案，而是根据前面的文字，猜测下一个最合适的词。",
		"",
		"## 生活里的例子",
		"- **填空**：看到「今天天气真」，很多人会接「好」。模型做的是同一件事，只是它把整段话都当成填空。",
		"- **学徒厨师**：他尝过成千上万道菜，所以知道盐后面常常是胡椒。他不是记住了你家今天的菜单。",
		"- **接龙**：每个人只加一句，还要接得上上一句。模型每次只加一小段，再把已经写成的内容当作新的上文。",
		"",
		"所以它会写得很流畅，也会在不知道的时候编得像真的。要核对的是事实，不是语气。",
	}, "\n")
}

func writingAnswer(question string) string {
	return strings.Join([]string{
		"## 结论",
		"把想法写成专业文字，先定读者和目的，再按「背景、判断、请求」三段写。删掉口语和重复。",
		"",
		"## 模板",
		"1. **背景**：发生了什么，对方需要知道的最少事实。",
		"2. **判断**：你的结论是什么。",
		"3. **请求**：希望对方做什么，什么时候完成。",
		"",
		"你的原始想法是：「" + oneLine(question) + "」。把其中的结论放在第一句，理由放在后面，对方就不用猜你要什么。",
		"",
		"把具体读者和场合发我，我可以直接改成一版可以发送的稿子。",
	}, "\n")
}

func analysisAnswer(question string) string {
	return strings.Join([]string{
		"## 结论",
		"从信息里提炼行动，先分开事实、解释和决定。事实可以核对，解释要标明是推断，决定必须对应一个负责人。",
		"",
		"## 步骤",
		"1. 列出原始事实，不写形容词。",
		"2. 每个结论都指回至少一条事实。对不上的删掉。",
		"3. 行动写成「谁，在什么时候，做什么，怎样算完成」。",
		"4. 标出还缺的数据。缺数据时不要假装已经有结论。",
		"",
		"当前材料是：「" + oneLine(question) + "」。把表格或原始记录贴过来，我可以按这个结构给你一版结论和行动清单。",
	}, "\n")
}

func explainAnswer(question string) string {
	return strings.Join([]string{
		"## 结论",
		"讲清一个复杂问题，用一个生活例子对应一个机制，不要一次堆三个比喻。",
		"",
		"## 讲法",
		"1. 用一句话定义它解决什么。",
		"2. 举一个能动手想象的例子。",
		"3. 说这个例子和真实机制哪里一样、哪里不一样。",
		"4. 给一个最小的正确例子。",
		"",
		"你想解释的是：「" + oneLine(question) + "」。告诉我听众是谁，我按这个讲法展开。",
	}, "\n")
}

func goGenericAnswer(question string) string {
	return strings.Join([]string{
		"## 结论",
		"这个问题可以落成一段能编译的 Go。先把输入、输出和错误写进函数签名，再补实现。",
		"",
		fence("go", "func Handle(ctx context.Context, input string) (string, error) {\n    if err := ctx.Err(); err != nil {\n        return \"\", err\n    }\n    input = strings.TrimSpace(input)\n    if input == \"\" {\n        return \"\", errors.New(\"输入不能为空\")\n    }\n    return input, nil\n}"),
		"",
		"你的问题是：「" + oneLine(question) + "」。如果把期望的输入输出发我，我可以把这段换成针对该问题的实现。",
	}, "\n")
}

func genericAnswer(question string) string {
	line := oneLine(question)
	if line == "" {
		line = "这件事"
	}
	return strings.Join([]string{
		"## 结论",
		"围绕「" + line + "」，先写清做成什么样，再拆成能当天完成的步骤。每一步都要有完成标准。",
		"",
		"## 步骤",
		"1. 用一句话写下目标和不做的范围。",
		"2. 拆成 3 到 5 个动作，每个动作都能单独检查。",
		"3. 先做最小的一版，确认方向再加深。",
		"4. 用事实核对结果，分清已经发生的和还只是推断。",
		"",
		"## 可以继续发给我的内容",
		"- 背景和限制",
		"- 你已经试过什么",
		"- 希望我交付的形式：步骤、代码或一版可以直接发送的文字",
		"",
		"我会按你补充的事实继续，不会把没给出的数据说成已经发生。",
	}, "\n")
}

func oneLine(question string) string {
	text := strings.TrimSpace(question)
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		text = text[:i]
	}
	text = strings.Join(strings.Fields(text), " ")
	rs := []rune(text)
	if len(rs) > 80 {
		return string(rs[:80]) + "…"
	}
	return text
}
