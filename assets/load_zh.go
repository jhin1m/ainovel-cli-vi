package assets

//check-han:ignore-file — dữ liệu chỉ dùng khi language=zh (giữ nguyên bản gốc upstream).

// simulationGuidance là chỉ dẫn phỏng tác bản tiếng Trung gốc.
const simulationGuidance = `## 仿写画像

当 novel_context 的 planning_memory 或 working_memory 中存在 simulation_profile 时，必须把它视为当前作品的仿写方向约束。{{role}} 应读取其中的 style、lexicon、plot_design、hook_design、pacing_density、reader_engagement 和 role_guidance。

使用原则：借鉴结构、节奏、钩子、信息释放和吸引读者的手法；不要复制原文句子、人物、地名、专有设定或固定桥段。若 simulation_profile 与用户显式要求冲突，优先服从用户要求。`

// zhOverrideHeading là tiêu đề đoạn ghi đè văn phong bản tiếng Trung gốc.
var zhOverrideHeading = overrideHeading{"## 用户全局文风覆盖（以下要求优先于项目默认）", "## 本书文风覆盖（以下要求优先于以上全部）"}
