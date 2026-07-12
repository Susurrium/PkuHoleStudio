import { useQuery } from '@tanstack/react-query'
import { Eye, EyeOff, ShieldCheck } from 'lucide-react'
import { useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import type { CourseDay } from '../lib/types'

const days: [string, keyof import('../lib/types').CourseScheduleRow][] = [['一', 'mon'], ['二', 'tue'], ['三', 'wed'], ['四', 'thu'], ['五', 'fri'], ['六', 'sat'], ['日', 'sun']]

export function CampusPage() {
	const [tab, setTab] = useState<'schedule' | 'scores'>('schedule')
	const [hidden, setHidden] = useState(true)
	const session = useQuery({ queryKey: ['campus-session'], queryFn: api.probeSession, retry: false })
	const schedule = useQuery({ queryKey: ['campus-schedule'], queryFn: api.campusSchedule, enabled: session.data?.can_read_online === true && tab === 'schedule', staleTime: 60_000 })
	const scores = useQuery({ queryKey: ['campus-scores'], queryFn: api.campusScores, enabled: session.data?.can_read_online === true && tab === 'scores', staleTime: 60_000 })
	if (session.isLoading) return <LoadingState label="正在验证在线会话…" />
	if (!session.data?.can_read_online) return <ErrorState error={new Error(session.data?.message || '请先在同步中心登录')} />
	return <><PageHeader eyebrow="CAMPUS" title="课表与成绩" description="数据仅在登录后实时读取并保留在当前页面内存，不写入资料库、归档、日志或 AI 上下文。" actions={<span className="badge gap-1"><ShieldCheck size={12} />仅在线读取</span>} /><div className="mb-5 flex flex-wrap items-center justify-between gap-3"><div className="inline-flex rounded-xl border border-line bg-white/50 p-1"><button className={`rounded-lg px-4 py-2 text-sm font-medium ${tab === 'schedule' ? 'bg-ink text-white' : 'text-ink-soft'}`} onClick={() => setTab('schedule')}>周课表</button><button className={`rounded-lg px-4 py-2 text-sm font-medium ${tab === 'scores' ? 'bg-ink text-white' : 'text-ink-soft'}`} onClick={() => setTab('scores')}>成绩</button></div>{tab === 'scores' && <button className="button-secondary" onClick={() => setHidden(!hidden)}>{hidden ? <Eye size={15} /> : <EyeOff size={15} />}{hidden ? '显示成绩' : '隐藏成绩'}</button>}</div>{tab === 'schedule' ? schedule.isLoading ? <LoadingState /> : schedule.error ? <ErrorState error={schedule.error} /> : <div className="panel overflow-auto"><table className="w-full min-w-[900px] border-collapse text-sm"><thead><tr><th className="border-b border-line p-3 text-left">节次</th>{days.map(([name]) => <th key={name} className="border-b border-line p-3 text-left">星期{name}</th>)}</tr></thead><tbody>{schedule.data?.map((row) => <tr key={row.time_num}><td className="border-b border-line/60 p-3 font-mono text-teal">{row.time_num}</td>{days.map(([name, key]) => <CourseCell key={name} value={row[key] as CourseDay} />)}</tr>)}</tbody></table></div> : scores.isLoading ? <LoadingState /> : scores.error ? <ErrorState error={scores.error} /> : <div className={hidden ? 'select-none blur-md' : ''}><section className="mb-5 grid gap-4 sm:grid-cols-4"><Metric label="GPA" value={scores.data?.gpa} /><Metric label="总学分" value={scores.data?.total_credit} /><Metric label="已修学分" value={scores.data?.passed_credit} /><Metric label="课程数" value={scores.data?.course_count} /></section><div className="panel overflow-auto"><table className="w-full min-w-[700px] text-sm"><thead><tr>{['学期', '课程', '类别', '学分', '成绩'].map((name) => <th key={name} className="border-b border-line p-3 text-left">{name}</th>)}</tr></thead><tbody>{scores.data?.scores.map((score, index) => <tr key={`${score.year_term}-${score.name}-${index}`}><td className="border-b border-line/60 p-3">{score.year_term}</td><td className="border-b border-line/60 p-3 font-medium">{score.name}</td><td className="border-b border-line/60 p-3">{score.category}</td><td className="border-b border-line/60 p-3">{score.credit}</td><td className="border-b border-line/60 p-3 font-semibold text-coral">{score.score}</td></tr>)}</tbody></table></div></div>}</>
}

function CourseCell({ value }: { value: CourseDay }) { return <td className="border-b border-line/60 p-3"><p className="font-medium">{value?.courseName || '—'}</p>{value?.parity && <p className="mt-1 text-xs text-ink-soft">{value.parity}</p>}</td> }
function Metric({ label, value }: { label: string; value?: string }) { return <div className="panel p-5"><p className="text-xs text-ink-soft">{label}</p><p className="mt-2 text-2xl font-semibold">{value || '—'}</p></div> }
