package notify

import (
	"math/rand/v2"
	"strings"
)

// Tagline pools keyed by sign-result status. Picked at random and appended
// to every email + Server酱 push so the notifications don't read like a
// build server. Adding to a pool is a one-line change — no template churn.
//
// Tone guide:
//   success: 轻松，带点表扬
//   already: 平淡，确认就好
//   exempt:  关怀向，认可请假/外宿是正当的
//   failed:  抱歉口吻，给指引（不要责怪用户）
//   tokenWarn: 提醒口吻，强调要尽快

var taglinesSuccess = []string{
	"今夜风高月黑，但你的签到稳如老狗 🐕",
	"打卡完成，可以放心刷手机了 📱",
	"系统替你完成了一件小事，去睡个好觉 😴",
	"嗯，又是顺利的一天。",
	"今天的回归记录 +1，朋友们都安然 👍",
	"今晚的电子代签已就位，请安心入睡 🌙",
	"不动声色地完成签到，是一种温柔。",
	"任务完成，明天还有更多签到等着 ✨",
	"安全打卡，宿舍 wifi 安好。",
	"小事一桩，让 wangui 来处理就好。",
}

var taglinesAlready = []string{
	"已经签过啦，那就早点休息 😌",
	"你比 wangui 还快一步 ⚡",
	"提前手签也很棒，我就负责兜底。",
	"勤快的孩子，连脚本都赶不上你。",
	"嗯，今天的回归记录已就位。",
}

var taglinesExempt = []string{
	"今天系统标了免签，安心放假吧 🌴",
	"请假在身？那就好好休整一下。",
	"节假日离校 / 走读模式，今晚无需打卡。",
	"今晚 wangui 不打扰你了，享受自由 🕊",
	"该歇就歇，签到这事改天再说。",
}

var taglinesFailed = []string{
	"今晚没签上，请尽快检查 Token / 宿舍楼配置 🛠",
	"签到失败 —— 别慌，22:30 前还可以手动补签。",
	"看来今晚出了点状况，admin 已收到告警。",
	"系统这次没顶住，让我们看看哪里漏了。",
	"失败不要紧，赶紧打开账号页换个新 Token 试试。",
}

var taglinesTokenWarn = []string{
	"Token 是短命的，别等过期才去刷 ⏰",
	"提早刷新一下，省得明晚白等。",
	"小提醒：再不刷新，wangui 就要请你手动签到了。",
	"Token 健康度告急，2 分钟刷一下省心一晚。",
	"系统不会替你扛过期，赶紧补水 ⚡",
}

// pick returns a random entry from the pool, or "" if the pool is empty.
// Note: math/rand/v2 is goroutine-safe.
func pick(pool []string) string {
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.IntN(len(pool))]
}

// taglineForSign returns a random tagline appropriate for a sign-result
// status. Falls back to an empty string for unknown statuses (skipped, etc.)
// so the email just doesn't get a footer line.
func taglineForSign(status string) string {
	switch status {
	case "success":
		return pick(taglinesSuccess)
	case "already":
		return pick(taglinesAlready)
	case "exempt":
		return pick(taglinesExempt)
	case "failed":
		return pick(taglinesFailed)
	}
	return ""
}

// taglineForTokenWarn returns a token-expiry-flavored tagline.
func taglineForTokenWarn() string {
	return pick(taglinesTokenWarn)
}

// withTagline appends a tagline to a body when one applies. Two newlines
// before so the line stands apart from the main content (markdown paragraph
// break in Server酱; visual gap in plain text email).
func withTagline(body, tagline string) string {
	tagline = strings.TrimSpace(tagline)
	if tagline == "" {
		return body
	}
	return body + "\n\n— " + tagline
}
