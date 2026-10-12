//check-han:ignore-file 本文件只在 language=zh 时使用，保留上游中文数据原样。

package rules

// systemDefaultsZH 是中文写作的机械基线（上游原值）。
//
// 数值迁自旧 assets/rules/default.md 的 front matter。阈值依据一并保留：
// 后段疲劳词（像一/沉默了/没有说话/X息）来自 196 章长跑产物实证——传统 AI 套话被前段
// 表灭绝后，模型转而把这些"节拍词"用到章均 5-7 次，阈值放宽以容忍正常使用。
func systemDefaultsZH() Structured {
	return Structured{
		// 定长固定串的 AI 套句；checker 字面子串匹配，带变量的模式（不是X而是Y）归语义层。
		ForbiddenPhrases: []string{"某种程度上", "值得注意的是", "不知为何", "五味杂陈"},
		FatigueWords: map[string]int{
			"不禁": 1, "竟然": 1, "仿佛": 2, "此外": 1, "然而": 2,
			"一丝": 2, "一抹": 2, "一缕": 2, "宛如": 1, "不由得": 1,
			"像一": 3, "沉默了": 2, "没有说话": 2, "几息": 3, "一息": 3, "数息": 2,
		},
	}
}

// SystemDefaultsZH 返回中文基线的副本，供 userrules 识别并迁移在 zh 下生成的旧快照。
func SystemDefaultsZH() Structured { return systemDefaultsZH() }

// homeRulesReadmeZH 是 language=zh 时首次引导写入 ~/.ainovel/rules/README.txt 的说明。
// 刻意用 .txt 后缀而非 .md——扫描只认 .md，这份说明不会被当成规则归一化。
const homeRulesReadmeZH = `这里放全局写作偏好，跨所有书生效。

新建一个 .md 文件（如 my-style.md），用大白话写要求就行——
不需要任何格式、不需要 YAML：

    # 角色
    - 主角林尘别写成圣母，外冷内热即可
    # 风格
    - 多用身体感知（指节发白）替代情绪标签（紧张）
    - 对话别太书面，每章 3000 字左右
    - 不要出现"某种程度上"这种 AI 腔

写完不用管格式：系统会用模型把这些自然语言要求归一化成结构化约束
（字数范围、禁用词、疲劳词阈值等），写作时自动遵循、提交时自动自检。

多个 .md 按文件名字典序合并；点开头的隐藏文件、非 .md 文件都会被忽略
（所以这份 README.txt 不会被当成规则）。

常见 AI 套句、疲劳词的机械基线已内置，开箱即用，不写也没关系。

加载优先级（高 → 低）：./.ainovel/rules/*.md（本书） > ~/.ainovel/rules/*.md（这里） > 内置默认
`
