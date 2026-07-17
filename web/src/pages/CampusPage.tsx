import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { CalendarDays, Eye, EyeOff, LogIn, RefreshCw, ShieldCheck } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { PageHeader } from '../components/PageHeader'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import type { CourseDay, CourseScheduleRow, CourseScore, ScoreSummary } from '../lib/types'
import { useOnlineSession, useSlowLoading } from '../features/online/session'

type CampusTab = 'schedule' | 'scores'
type DayKey = keyof Omit<CourseScheduleRow, 'time_num'>

const days: { short: string; label: string; key: DayKey }[] = [
	{ short: '一', label: '星期一', key: 'mon' },
	{ short: '二', label: '星期二', key: 'tue' },
	{ short: '三', label: '星期三', key: 'wed' },
	{ short: '四', label: '星期四', key: 'thu' },
	{ short: '五', label: '星期五', key: 'fri' },
	{ short: '六', label: '星期六', key: 'sat' },
	{ short: '日', label: '星期日', key: 'sun' },
]

export function CampusPage() {
	const [params, setParams] = useSearchParams()
	const tab: CampusTab = params.get('view') === 'scores' ? 'scores' : 'schedule'
	const todayIndex = (new Date().getDay() + 6) % 7
	const [selectedDay, setSelectedDay] = useState<DayKey>(days[todayIndex].key)
	const [scoresRequested, setScoresRequested] = useState(false)
	const [scoresVisible, setScoresVisible] = useState(false)
	const session = useOnlineSession()
	const slowSession = useSlowLoading(session.isLoading)
	const online = session.data?.can_read_online === true
	const schedule = useQuery({ queryKey: ['campus-schedule'], queryFn: api.campusSchedule, enabled: online && tab === 'schedule', staleTime: 60_000 })
	const scores = useQuery({ queryKey: ['campus-scores'], queryFn: api.campusScores, enabled: online && tab === 'scores' && scoresRequested, staleTime: 60_000 })
	const setTab = (next: CampusTab) => {
		const nextParams = new URLSearchParams(params)
		if (next === 'scores') nextParams.set('view', 'scores'); else nextParams.delete('view')
		setParams(nextParams, { replace: true })
		if (next !== 'scores') setScoresVisible(false)
	}

	return <>
		<PageHeader
			eyebrow="CAMPUS"
			title="课表与成绩"
			description="数据只在登录后按需实时读取，并仅保留在当前页面内存中。"
			actions={<span className="badge gap-1"><ShieldCheck size={12} />不写入资料库与 AI 上下文</span>}
		/>
		{session.isLoading ? <LoadingState label={slowSession ? '正在连接树洞，可能需要几秒…' : '正在验证在线会话…'} /> : session.error ? <ErrorState error={session.error} /> : !online ? <EmptyState
			title="登录后使用校园信息"
			description={session.data?.message || '课表和成绩需要通过当前本机会话实时读取。'}
			action={<Link className="button-primary" to="/sync"><LogIn size={16} />前往同步中心登录</Link>}
		/> : <>
			<div className="mb-5 flex flex-wrap items-center justify-between gap-3">
				<nav className="grid min-w-[280px] grid-cols-2 rounded-xl border border-line bg-white/45 p-1" aria-label="校园数据类型">
					<CampusTabButton active={tab === 'schedule'} label="周课表" onClick={() => setTab('schedule')} />
					<CampusTabButton active={tab === 'scores'} label="成绩" onClick={() => setTab('scores')} />
				</nav>
				{tab === 'schedule' && <button className="button-secondary" disabled={schedule.isFetching} onClick={() => schedule.refetch()}><RefreshCw size={15} />{schedule.isFetching ? '刷新中…' : '刷新课表'}</button>}
				{tab === 'scores' && scoresRequested && scoresVisible && <div className="flex gap-2"><button className="button-secondary" disabled={scores.isFetching} onClick={() => scores.refetch()}><RefreshCw size={15} />刷新</button><button className="button-secondary" onClick={() => setScoresVisible(false)}><EyeOff size={15} />隐藏成绩</button></div>}
			</div>
			{tab === 'schedule' ? <SchedulePanel query={schedule} selectedDay={selectedDay} setSelectedDay={setSelectedDay} todayIndex={todayIndex} /> : !scoresVisible ? <ScorePrivacyGate loaded={scoresRequested} reveal={() => { setScoresRequested(true); setScoresVisible(true) }} /> : <ScoresPanel query={scores} />}
		</>}
	</>
}

function CampusTabButton({ active, label, onClick }: { active: boolean; label: string; onClick: () => void }) {
	return <button className={`rounded-lg px-4 py-2.5 text-sm font-medium transition ${active ? 'bg-ink text-white shadow-sm' : 'text-ink-soft hover:bg-white/60 hover:text-ink'}`} aria-pressed={active} onClick={onClick}>{label}</button>
}

function SchedulePanel({ query, selectedDay, setSelectedDay, todayIndex }: { query: UseQueryResult<CourseScheduleRow[]>; selectedDay: DayKey; setSelectedDay: (day: DayKey) => void; todayIndex: number }) {
	if (query.isLoading) return <LoadingState label="正在读取课表…" />
	if (query.error) return <ErrorState error={query.error} />
	const rows = query.data ?? []
	if (!rows.length) return <EmptyState title="当前没有课表数据" description="请确认教务系统中已有本学期课表，或稍后刷新重试。" action={<CalendarDays size={20} />} />
	const currentDay = days.find((day) => day.key === selectedDay) ?? days[0]
	const mobileRows = rows.filter((row) => row[selectedDay]?.courseName)
	return <>
		<section className="md:hidden">
			<div className="mb-4 grid grid-cols-7 gap-1 rounded-xl border border-line bg-white/45 p-1" aria-label="选择星期">
				{days.map((day, index) => <button key={day.key} className={`relative rounded-lg py-2 text-xs font-medium ${selectedDay === day.key ? 'bg-ink text-white' : 'text-ink-soft'}`} aria-pressed={selectedDay === day.key} aria-label={day.label} onClick={() => setSelectedDay(day.key)}>{day.short}{index === todayIndex && <span className="absolute inset-x-0 bottom-0.5 mx-auto size-1 rounded-full bg-coral" />}</button>)}
			</div>
			<div className="mb-3 flex items-center justify-between"><h2 className="text-lg font-semibold">{currentDay.label}</h2><span className="badge">{mobileRows.length} 节课</span></div>
			<div className="grid gap-3">{mobileRows.length ? mobileRows.map((row) => <MobileCourse key={row.time_num} time={row.time_num} course={row[selectedDay]} />) : <div className="panel p-8 text-center text-sm text-ink-soft">这一天没有课程</div>}</div>
		</section>
		<div className="panel hidden overflow-auto md:block">
			<table className="w-full min-w-[900px] border-collapse text-sm">
				<thead><tr><th className="sticky left-0 z-10 border-b border-line bg-[#f9f6f0] p-3 text-left">节次</th>{days.map((day, index) => <th key={day.key} className={`border-b border-line p-3 text-left ${index === todayIndex ? 'bg-teal-soft/35 text-teal' : ''}`}>{day.label}{index === todayIndex && <span className="ml-2 text-[10px]">今天</span>}</th>)}</tr></thead>
				<tbody>{rows.map((row) => <tr key={row.time_num}><td className="sticky left-0 border-b border-line/60 bg-[#f9f6f0] p-3 font-mono text-teal">{row.time_num}</td>{days.map((day, index) => <CourseCell key={day.key} value={row[day.key]} today={index === todayIndex} />)}</tr>)}</tbody>
			</table>
		</div>
	</>
}

function ScorePrivacyGate({ loaded, reveal }: { loaded: boolean; reveal: () => void }) {
	return <section className="panel flex min-h-72 flex-col items-center justify-center border-teal/25 p-8 text-center">
		<div className="grid size-14 place-items-center rounded-2xl bg-teal-soft text-teal"><ShieldCheck size={25} /></div>
		<h2 className="mt-5 text-xl font-semibold">成绩默认保持隐藏</h2>
		<p className="mt-2 max-w-lg text-sm leading-6 text-ink-soft">只有点击下方按钮后才会{loaded ? '重新显示已读取的数据' : '向教务系统请求成绩'}。切换页面后会自动隐藏，页面不会把成绩写入资料库、日志、归档或 AI 上下文。</p>
		<button className="button-primary mt-6" onClick={reveal}><Eye size={16} />{loaded ? '重新显示成绩' : '加载并显示成绩'}</button>
	</section>
}

function ScoresPanel({ query }: { query: UseQueryResult<ScoreSummary> }) {
	if (query.isLoading) return <LoadingState label="正在安全读取成绩…" />
	if (query.error) return <ErrorState error={query.error} />
	const data = query.data
	if (!data) return <EmptyState title="没有读取到成绩" description="请稍后刷新，或确认教务系统当前可访问。" />
	return <div>
		<section className="mb-5 grid grid-cols-2 gap-3 lg:grid-cols-4"><Metric label="GPA" value={data.gpa} /><Metric label="总学分" value={data.total_credit} /><Metric label="已修学分" value={data.passed_credit} /><Metric label="课程数" value={data.course_count} /></section>
		{data.scores.length ? <>
			<div className="grid gap-3 md:hidden">{data.scores.map((score, index) => <MobileScore key={`${score.year_term}-${score.name}-${index}`} score={score} />)}</div>
			<div className="panel hidden overflow-auto md:block"><table className="w-full min-w-[700px] text-sm"><thead><tr>{['学期', '课程', '类别', '学分', '成绩'].map((name) => <th key={name} className="border-b border-line p-3 text-left">{name}</th>)}</tr></thead><tbody>{data.scores.map((score, index) => <tr key={`${score.year_term}-${score.name}-${index}`}><td className="border-b border-line/60 p-3">{score.year_term}</td><td className="border-b border-line/60 p-3 font-medium">{score.name}</td><td className="border-b border-line/60 p-3 text-ink-soft">{score.category}</td><td className="border-b border-line/60 p-3">{score.credit}</td><td className="border-b border-line/60 p-3 text-lg font-semibold text-coral">{score.score}</td></tr>)}</tbody></table></div>
		</> : <EmptyState title="暂无课程成绩" description="当前账户没有返回可显示的成绩记录。" />}
	</div>
}

function MobileCourse({ time, course }: { time: string; course: CourseDay }) {
	return <article className="panel flex items-start gap-4 p-4"><span className="badge shrink-0">{time}</span><div><h3 className="font-semibold">{course.courseName}</h3>{course.parity && <p className="mt-1 text-xs text-ink-soft">{course.parity}</p>}</div></article>
}

function CourseCell({ value, today }: { value: CourseDay; today: boolean }) {
	return <td className={`border-b border-line/60 p-3 align-top ${today ? 'bg-teal-soft/15' : ''}`}><p className="font-medium">{value?.courseName || '—'}</p>{value?.parity && <p className="mt-1 text-xs text-ink-soft">{value.parity}</p>}</td>
}

function MobileScore({ score }: { score: CourseScore }) {
	return <article className="panel p-4"><div className="flex items-start justify-between gap-4"><div><h3 className="font-semibold">{score.name}</h3><p className="mt-1 text-xs text-ink-soft">{score.year_term} · {score.category || '未分类'} · {score.credit} 学分</p></div><span className="text-xl font-semibold text-coral">{score.score}</span></div></article>
}

function Metric({ label, value }: { label: string; value?: string }) {
	return <div className="panel p-4 sm:p-5"><p className="text-xs text-ink-soft">{label}</p><p className="mt-2 text-2xl font-semibold">{value || '—'}</p></div>
}
