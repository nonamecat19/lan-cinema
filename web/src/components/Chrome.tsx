import type { ButtonHTMLAttributes, ReactNode } from 'react'
import type { Navigate } from '../hooks/useRoute'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  focusKey: string
}

export function Button({ children, className = '', focusKey, ...props }: ButtonProps) {
  return <button type="button" data-focusable="true" data-focus-key={focusKey} className={`button ${className}`} {...props}>{children}</button>
}

type HeaderProps = { navigate: Navigate; section?: string } & ({ back: true; onBack: () => void } | { back?: false; onBack?: never })

export function Header({ navigate, back = false, onBack, section = 'YOUR LIBRARY' }: HeaderProps) {
  return <header className="topbar">
    <div className="brand" aria-label="Frame home"><span className="brand-icon">F</span><span>FRAME<span className="brand-period">.</span></span></div>
    {back && <Button className="back-button secondary" focusKey="header-back" onClick={onBack}>← <span>Library</span></Button>}
    <span className="topbar-meta">{section}</span>
    {!back && <Button className="header-downloads secondary" focusKey="open-downloads" onClick={() => navigate('/downloads')}><span aria-hidden="true">↓</span> Downloads</Button>}
  </header>
}

export function Footer({ message = 'MAKE YOURSELF AT HOME' }: { message?: string }) {
  return <footer className="footer-note"><span>FRAME / LOCAL CINEMA</span><span>{message}</span><span>ARROWS OR 2·4·6·8 NAVIGATE&nbsp;&nbsp; · &nbsp;&nbsp; OK / 5 SELECT&nbsp;&nbsp; · &nbsp;&nbsp; BACK / 0 RETURN</span></footer>
}

export function StateScreen({ eyebrow, title, description, action, loading = false }: { eyebrow: string; title: string; description?: string; action?: ReactNode; loading?: boolean }) {
  return <main className="state-screen">
    {loading ? <div className="loader" aria-hidden="true" /> : <div className="state-mark" aria-hidden="true">F</div>}
    <span className="eyebrow">{eyebrow}</span>
    <h1>{title}</h1>
    {description && <p>{description}</p>}
    {action}
  </main>
}
