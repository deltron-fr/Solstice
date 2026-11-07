import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './app/App';
import { SolanaProvider } from './components/layout/SolanaProvider';
import './styles/global.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode><SolanaProvider><App /></SolanaProvider></React.StrictMode>,
);
