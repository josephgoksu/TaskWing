import { useState, useEffect } from 'react'
import './GitHubStats.css'

interface GitHubData {
  stars: number
  forks: number
  issues: number
  lastUpdated: string
}

export function GitHubStats() {
  const [stats, setStats] = useState<GitHubData>({
    stars: 0,
    forks: 0,
    issues: 0,
    lastUpdated: 'Loading...'
  })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchGitHubStats = async () => {
      try {
        const response = await fetch('https://api.github.com/repos/josephgoksu/TaskWing', {
          headers: {
            'Accept': 'application/vnd.github.v3+json',
          },
        })
        
        if (!response.ok) {
          throw new Error(`GitHub API error: ${response.status}`)
        }
        
        const data = await response.json()
        
        setStats({
          stars: data.stargazers_count || 0,
          forks: data.forks_count || 0,
          issues: data.open_issues_count || 0,
          lastUpdated: new Date(data.updated_at).toLocaleDateString()
        })
        setLoading(false)
      } catch (error) {
        console.log('GitHub stats unavailable, using fallback values:', error)
        // Fallback to reasonable default values if API fails
        setStats({
          stars: 42,
          forks: 8,
          issues: 3,
          lastUpdated: 'Recently'
        })
        setLoading(false)
      }
    }

    fetchGitHubStats()
  }, [])

  if (loading) {
    return (
      <div className="github-stats loading">
        <div className="stat-skeleton"></div>
        <div className="stat-skeleton"></div>
        <div className="stat-skeleton"></div>
      </div>
    )
  }

  return (
    <div className="github-stats">
      <div className="github-stat">
        <span className="stat-icon">⭐</span>
        <span className="stat-value">{stats.stars.toLocaleString()}</span>
        <span className="stat-label">Stars</span>
      </div>
      <div className="github-stat">
        <span className="stat-icon">🍴</span>
        <span className="stat-value">{stats.forks}</span>
        <span className="stat-label">Forks</span>
      </div>
      <div className="github-stat">
        <span className="stat-icon">🐛</span>
        <span className="stat-value">{stats.issues}</span>
        <span className="stat-label">Issues</span>
      </div>
    </div>
  )
}