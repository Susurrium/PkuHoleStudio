import { Bot, LockKeyhole, Search, Sparkles } from 'lucide-react'
import { PageHeader } from '../components/PageHeader'

export function AIPage() {
  return <>
    <PageHeader eyebrow="LOCAL RESEARCH" title="AI 研究台" description="让模型围绕本地资料回答问题，并把每个结论连接回具体 PID/CID。Provider 与检索 Agent 将在下一阶段启用。" />
    <section className="panel overflow-hidden">
      <div className="grid min-h-[420px] place-items-center bg-[radial-gradient(circle_at_75%_20%,rgba(228,101,79,.13),transparent_36%),radial-gradient(circle_at_20%_85%,rgba(40,127,120,.14),transparent_35%)] p-8 text-center">
        <div className="max-w-xl"><div className="mx-auto grid size-16 place-items-center rounded-2xl bg-ink text-white shadow-[6px_6px_0_#e4654f]"><Bot size={28} /></div><h2 className="mt-7 text-2xl font-semibold tracking-tight">可追溯的本地问答，而不是黑盒聊天</h2><p className="mt-3 text-sm leading-7 text-ink-soft">第一版将提供选中内容问答、本地自动检索和课程分析。回答会保存搜索轨迹、模型以及引用的帖子和评论。</p><div className="mt-7 grid gap-3 text-left sm:grid-cols-3"><Feature icon={Search} text="最多五轮本地检索" /><Feature icon={Sparkles} text="课程与教师对比" /><Feature icon={LockKeyhole} text="实时搜索默认关闭" /></div><span className="badge mt-8">Phase 4 即将启用</span></div>
      </div>
    </section>
  </>
}

function Feature({ icon: Icon, text }: { icon: typeof Search; text: string }) { return <div className="rounded-xl border border-line bg-white/55 p-4"><Icon className="text-teal" size={18} /><p className="mt-3 text-xs font-medium leading-5">{text}</p></div> }
