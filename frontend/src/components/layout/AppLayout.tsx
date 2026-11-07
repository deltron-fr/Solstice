import { NavLink, Outlet } from 'react-router-dom';
import { WalletButton } from '../../features/wallet/WalletButton';

export function AppLayout() {
  return (
    <div className="app-shell">
      <header className="topbar">
        <NavLink className="brand" to="/" aria-label="Pact home"><span className="brand-mark">p</span> pact</NavLink>
        <nav className="main-nav" aria-label="Main navigation">
          <NavLink to="/">Explore</NavLink>
          <NavLink to="/seller">For sellers</NavLink>
          <NavLink to="/orders">My orders</NavLink>
          <NavLink to="/messages">Messages</NavLink>
        </nav>
        <div className="topbar-actions"><a className="text-link" href="#how-it-works">How it works</a><WalletButton /></div>
      </header>
      <main><Outlet /></main>
      <footer className="footer"><span>© 2026 Pact Marketplace</span><span>Good business, built on trust.</span></footer>
    </div>
  );
}
