<script lang="ts">
  import { taskStats } from '../lib/store';
  import { GetRunningCount } from '../../wailsjs/go/main/App';

  let runningCount = 0;

  // Poll running count
  const interval = setInterval(async () => {
    try {
      runningCount = await GetRunningCount();
    } catch (_) { /* polling failure is non-critical */ }
  }, 2000);

  import { onDestroy } from 'svelte';
  onDestroy(() => clearInterval(interval));
</script>

<header>
  <div class="header-left">
    <h1>Web Automation</h1>
    <span class="subtitle">Go + Wails + chromedp</span>
  </div>
  <div class="header-stats">
    <div class="stat">
      <span class="stat-value">{$taskStats.total}</span>
      <span class="stat-label">Total</span>
    </div>
    <div class="stat running">
      <span class="stat-value">{runningCount}</span>
      <span class="stat-label">Running</span>
    </div>
    <div class="stat success">
      <span class="stat-value">{$taskStats.completed}</span>
      <span class="stat-label">Done</span>
    </div>
    <div class="stat danger">
      <span class="stat-value">{$taskStats.failed}</span>
      <span class="stat-label">Failed</span>
    </div>
  </div>
</header>

<style>
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 20px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .header-left {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }
  h1 {
    font-size: 18px;
    font-weight: 700;
    margin: 0;
  }
  .subtitle {
    font-size: 12px;
    color: var(--text-muted);
  }
  .header-stats {
    display: flex;
    gap: 20px;
  }
  .stat {
    text-align: center;
  }
  .stat-value {
    display: block;
    font-size: 20px;
    font-weight: 700;
  }
  .stat-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .running .stat-value { color: var(--accent); }
  .success .stat-value { color: var(--success); }
  .danger .stat-value { color: var(--danger); }
</style>
