import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Sidebar from './components/Sidebar'
import TokenGate from './components/TokenGate'
import Dashboard from './pages/Dashboard'
import JobDetail from './pages/JobDetail'
import Metrics from './pages/Metrics'
import Campaigns from './pages/Campaigns'
import CampaignDetail from './pages/CampaignDetail'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      retry: 1,
    }
  }
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <TokenGate>
      <BrowserRouter>
        <div className="flex h-screen bg-background text-on-surface font-body-md text-body-md">
          <Sidebar />
          <main className="flex-1 overflow-auto p-space-xl">
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/jobs/:jobId" element={<JobDetail />} />
              <Route path="/campaigns" element={<Campaigns />} />
              <Route path="/campaigns/:campaignId" element={<CampaignDetail />} />
              <Route path="/metrics" element={<Metrics />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
      </TokenGate>
    </QueryClientProvider>
  )
}
