import { useQuery } from '@tanstack/react-query'
import { Archive, ArrowRight, Download, Import, Radio, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useUIStore, type Workspace } from '../store/ui'

export const ONBOARDING_STORAGE_KEY = 'pkustudio:onboarding:v1'
const onboardingRestartEvent = 'pkustudio:restart-onboarding'

export function restartOnboarding() {
  try { window.localStorage.removeItem(ONBOARDING_STORAGE_KEY) } catch { /* best effort */ }
  window.dispatchEvent(new Event(onboardingRestartEvent))
}

function onboardingCompleted() {
  try { return window.localStorage.getItem(ONBOARDING_STORAGE_KEY) === 'completed' } catch { return false }
}

function rememberOnboardingCompleted() {
  try { window.localStorage.setItem(ONBOARDING_STORAGE_KEY, 'completed') } catch { /* best effort */ }
}

export function OnboardingGuide() {
  const navigate = useNavigate()
  const location = useLocation()
  const layoutPreset = useUIStore((state) => state.layoutPreset)
  const [open, setOpen] = useState(false)
  const [step, setStep] = useState(0)
  const [destination, setDestination] = useState<Workspace>('library')
  const heading = useRef<HTMLHeadingElement>(null)
  const dialog = useRef<HTMLElement>(null)
  const health = useQuery({
    queryKey: ['health'],
    queryFn: api.health,
    enabled: layoutPreset === 'studio' && location.pathname === '/' && !onboardingCompleted(),
    staleTime: 30_000,
  })

  useEffect(() => {
    if (layoutPreset === 'studio' && location.pathname === '/' && health.data?.posts === 0 && !onboardingCompleted()) setOpen(true)
  }, [health.data?.posts, layoutPreset, location.pathname])

  useEffect(() => {
    const restart = () => { setStep(0); setDestination('library'); setOpen(true) }
    window.addEventListener(onboardingRestartEvent, restart)
    return () => window.removeEventListener(onboardingRestartEvent, restart)
  }, [])

  useEffect(() => {
    if (!open || layoutPreset !== 'studio') return
    heading.current?.focus()
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') complete()
      if (event.key !== 'Tab') return
      const focusable = Array.from(dialog.current?.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])') ?? [])
      if (!focusable.length) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && (document.activeElement === first || document.activeElement === heading.current || !dialog.current?.contains(document.activeElement))) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.style.overflow = previousOverflow
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [open, layoutPreset])

  function complete(target?: Workspace) {
    rememberOnboardingCompleted()
    setOpen(false)
    if (target === 'online') navigate('/online')
    if (target === 'library') navigate('/imports')
  }

  if (!open || layoutPreset !== 'studio') return null

  return <div className="fixed inset-0 z-[90] grid place-items-center overflow-y-auto bg-ink/45 p-4 backdrop-blur-sm">
    <section ref={dialog} className="my-auto w-full max-w-3xl rounded-3xl border border-line bg-paper p-5 shadow-2xl sm:p-8" role="dialog" aria-modal="true" aria-labelledby="onboarding-title">
      <div className="flex items-start justify-between gap-4"><div><p className="eyebrow">WELCOME · {step + 1}/2</p><h2 ref={heading} tabIndex={-1} id="onboarding-title" className="mt-2 text-2xl font-semibold outline-none sm:text-3xl">{step === 0 ? '先选择你现在要做的事' : '理解在线与本地的关系'}</h2></div><button className="button-secondary !size-9 !p-0" aria-label="关闭并不再自动显示入门引导" onClick={() => complete()}><X size={16} /></button></div>

      {step === 0 ? <>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-ink-soft">PkuHoleStudio 有两个相互独立、随时可切换的工作区。选择当前需求即可，以后不会限制你的使用方式。</p>
        <div className="mt-6 grid gap-4 sm:grid-cols-2">
          <ChoiceCard selected={destination === 'online'} icon={Radio} title="在线树洞" description="实时浏览最新、热榜和关注，登录后可以发洞、评论和互动。" note="在线内容不会自动保存到本机" onClick={() => setDestination('online')} />
          <ChoiceCard selected={destination === 'library'} icon={Archive} title="本地资料库" description="导入任何兼容归档；也可选用独立 Toolkit 从官方树洞导出，然后离线搜索、标记和整理。" note="不要求登录 Studio，也不依赖 Toolkit 运行" onClick={() => setDestination('library')} />
        </div>
      </> : <>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-ink-soft">在线模式用于发现和互动；只有明确保存或同步后，内容才会进入本地资料库。</p>
        <div className="mt-6 grid gap-3 sm:grid-cols-[1fr_auto_1fr_auto_1fr] sm:items-center">
          <FlowStep icon={Radio} title="在线发现" description="浏览、热榜、关注" />
          <ArrowRight className="mx-auto hidden text-ink-soft sm:block" size={18} />
          <FlowStep icon={Download} title="明确保存" description="单个或批量保存" />
          <ArrowRight className="mx-auto hidden text-ink-soft sm:block" size={18} />
          <FlowStep icon={Import} title="本地整理" description="标签、项目、导出" />
        </div>
        <div className="mt-5 rounded-2xl border border-teal/25 bg-teal-soft/30 p-4 text-sm leading-6 text-ink-soft">左侧工作区开关可随时切换，也可以使用 <kbd className="rounded border border-line bg-white px-1.5 py-0.5 font-mono text-xs">Alt+1</kbd> 打开在线树洞、<kbd className="rounded border border-line bg-white px-1.5 py-0.5 font-mono text-xs">Alt+2</kbd> 返回本地资料库。</div>
      </>}

      <div className="mt-7 flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between"><button className="button-secondary" onClick={() => complete()}>不再自动显示</button><div className="flex gap-2">{step > 0 && <button className="button-secondary" onClick={() => setStep(0)}>上一步</button>}{step === 0 ? <button className="button-primary" onClick={() => setStep(1)}>继续<ArrowRight size={15} /></button> : <button className="button-primary" onClick={() => complete(destination)}>{destination === 'online' ? '进入在线树洞' : '前往导入资料'}<ArrowRight size={15} /></button>}</div></div>
    </section>
  </div>
}

function ChoiceCard({ selected, icon: Icon, title, description, note, onClick }: { selected: boolean; icon: typeof Radio; title: string; description: string; note: string; onClick: () => void }) {
  return <button type="button" aria-pressed={selected} className={`rounded-2xl border p-5 text-left transition ${selected ? 'border-teal bg-teal-soft/35 ring-2 ring-teal/15' : 'border-line bg-white/45 hover:border-teal/40 hover:bg-white/75'}`} onClick={onClick}><div className={`grid size-11 place-items-center rounded-xl ${selected ? 'bg-teal text-white' : 'bg-paper-deep text-teal'}`}><Icon size={20} /></div><p className="mt-4 text-lg font-semibold">{title}</p><p className="mt-2 text-sm leading-6 text-ink-soft">{description}</p><p className="mt-3 text-xs font-medium text-coral">{note}</p></button>
}

function FlowStep({ icon: Icon, title, description }: { icon: typeof Radio; title: string; description: string }) {
  return <div className="rounded-2xl border border-line bg-white/50 p-4 text-center"><div className="mx-auto grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={18} /></div><p className="mt-3 text-sm font-semibold">{title}</p><p className="mt-1 text-xs text-ink-soft">{description}</p></div>
}
