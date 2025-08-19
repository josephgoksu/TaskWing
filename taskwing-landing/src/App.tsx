import { useState, useEffect } from 'react'
import './App.css'
import './components/Animations.css'
import { TypewriterTerminal } from './components/TypewriterTerminal'
import { CookieConsent } from './components/CookieConsent'
import { CommunitySection } from './components/CommunitySection'
import { InstallationWizard } from './components/InstallationWizard'
import { FAQSection } from './components/FAQSection'
import { FeatureCard } from './components/FeatureCard'
import { DarkModeToggle } from './components/DarkModeToggle'
import { GitHubStats } from './components/GitHubStats'
import { useAnalytics, useDownloadTracking, useButtonTracking, useEngagementTracking } from './hooks/useAnalytics'
import { useParallaxEffect } from './hooks/useScrollAnimations'

function App() {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false)
  
  // Parallax ref for hero section
  const parallaxRef = useParallaxEffect(0.3)
  
  // Close mobile menu when clicking outside or on ESC key
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Element
      if (isMobileMenuOpen && !target.closest('.nav-container')) {
        setIsMobileMenuOpen(false)
      }
    }
    
    const handleEscKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && isMobileMenuOpen) {
        setIsMobileMenuOpen(false)
      }
    }
    
    if (isMobileMenuOpen) {
      document.addEventListener('click', handleClickOutside)
      document.addEventListener('keydown', handleEscKey)
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = 'unset'
    }
    
    return () => {
      document.removeEventListener('click', handleClickOutside)
      document.removeEventListener('keydown', handleEscKey)
      document.body.style.overflow = 'unset'
    }
  }, [isMobileMenuOpen])
  // Initialize analytics
  useAnalytics(import.meta.env.VITE_GA_MEASUREMENT_ID)
  useEngagementTracking()
  
  // Analytics hooks
  const { trackDownload } = useDownloadTracking()
  const { trackClick } = useButtonTracking()
  // const { createExperiment } = useAbTesting() // Disabled A/B testing
  
  // A/B test disabled - using fixed CTA text
  // const heroCtaVariant = createExperiment('hero_cta_text', [
  //   'Download TaskWing',
  //   'Get TaskWing Free',
  //   'Start Using TaskWing'
  // ], 0.8) // 80% of users in experiment
  // Define multiple demo sequences for the hero terminal
  const heroSequences = [
    {
      lines: [
        { type: 'command' as const, text: 'taskwing add "Implement user authentication"', delay: 120 },
        { type: 'output' as const, text: '✔ Task #42 created: Implement user authentication', delay: 60 },
        { type: 'output' as const, text: '➡ Run `taskwing list` to view your tasks', delay: 40 },
        { type: 'command' as const, text: 'taskwing list', delay: 100 },
        { type: 'output' as const, text: '┌──────────────────────────────────────────┐', delay: 30 },
        { type: 'output' as const, text: '│ ID │ Title                    │ Status   │', delay: 30 },
        { type: 'output' as const, text: '├──────────────────────────────────────────┤', delay: 30 },
        { type: 'output' as const, text: '│ 42 │ Implement user auth      │ pending  │', delay: 40 },
        { type: 'output' as const, text: '└──────────────────────────────────────────┘', delay: 30 }
      ],
      pauseAfter: 3000
    },
    {
      lines: [
        { type: 'command' as const, text: 'taskwing current set 42', delay: 110 },
        { type: 'output' as const, text: '✔ Current task set to #42: Implement user authentication', delay: 60 },
        { type: 'command' as const, text: 'taskwing mcp', delay: 90 },
        { type: 'output' as const, text: '🤖 MCP server started on localhost:3001', delay: 50 },
        { type: 'output' as const, text: '➡ AI tools can now manage your tasks', delay: 40 },
        { type: 'command' as const, text: 'taskwing done 42', delay: 100 },
        { type: 'output' as const, text: '🎉 Task #42 completed! Well done.', delay: 60 }
      ],
      pauseAfter: 3000
    },
    {
      lines: [
        { type: 'command' as const, text: 'taskwing add "Fix database migration" --priority high', delay: 140 },
        { type: 'output' as const, text: '✔ Task #43 created: Fix database migration', delay: 50 },
        { type: 'output' as const, text: '  Priority: high', delay: 30 },
        { type: 'output' as const, text: '  Created: 2025-08-19 16:45', delay: 30 },
        { type: 'command' as const, text: 'taskwing list --priority high', delay: 100 },
        { type: 'output' as const, text: '┌──────────────────────────────────────────┐', delay: 30 },
        { type: 'output' as const, text: '│ ID │ Title                    │ Priority │', delay: 30 },
        { type: 'output' as const, text: '├──────────────────────────────────────────┤', delay: 30 },
        { type: 'output' as const, text: '│ 43 │ Fix database migration   │ high     │', delay: 40 },
        { type: 'output' as const, text: '└──────────────────────────────────────────┘', delay: 30 }
      ],
      pauseAfter: 3500
    }
  ]

  // Define demo sequences for the CLI demo section
  const cliDemoSequences = [
    {
      lines: [
        { type: 'command' as const, text: 'taskwing list --status pending', delay: 120 },
        { type: 'output' as const, text: '┌──────────────────────────────────────────┐', delay: 40 },
        { type: 'output' as const, text: '│ ID │ Title                    │ Priority │', delay: 40 },
        { type: 'output' as const, text: '├──────────────────────────────────────────┤', delay: 40 },
        { type: 'output' as const, text: '│ 44 │ Fix authentication bug   │ high     │', delay: 50 },
        { type: 'output' as const, text: '│ 45 │ Update documentation     │ medium   │', delay: 50 },
        { type: 'output' as const, text: '└──────────────────────────────────────────┘', delay: 40 },
        { type: 'output' as const, text: '➡ Use `taskwing current set <id>` to start working', delay: 60 }
      ],
      pauseAfter: 4000
    }
  ]

  return (
    <div className="app">
      <a href="#main" className="skip-link">Skip to main content</a>
      
      {/* Navigation */}
      <nav className="navbar" role="navigation" aria-label="Main navigation">
        <div className="nav-container">
          <a href="#" className="nav-logo" aria-label="TaskWing home">
            <span className="logo-icon" aria-hidden="true">🪶</span>
            <span className="logo-text">TaskWing</span>
          </a>
          
          <button 
            className="mobile-menu-toggle"
            onClick={(e) => {
              e.stopPropagation()
              setIsMobileMenuOpen(!isMobileMenuOpen)
            }}
            aria-expanded={isMobileMenuOpen}
            aria-controls="nav-links"
            aria-label="Toggle navigation menu"
          >
            {isMobileMenuOpen ? '✕' : '☰'}
          </button>
          
          <div 
            id="nav-links"
            className={`nav-links ${isMobileMenuOpen ? 'open' : ''}`}
          >
            <a 
              href="#features"
              onClick={() => {
                trackClick('Features', 'navigation')
                setIsMobileMenuOpen(false)
              }}
            >
              Features
            </a>
            <a 
              href="#getting-started"
              onClick={() => {
                trackClick('Getting Started', 'navigation')
                setIsMobileMenuOpen(false)
              }}
            >
              Getting Started
            </a>
            <a 
              href="#faq"
              onClick={() => {
                trackClick('FAQ', 'navigation')
                setIsMobileMenuOpen(false)
              }}
            >
              FAQ
            </a>
            <a 
              href="https://github.com/josephgoksu/TaskWing" 
              target="_blank" 
              rel="noopener noreferrer"
              onClick={() => {
                trackClick('GitHub', 'navigation', 'https://github.com/josephgoksu/TaskWing')
                setIsMobileMenuOpen(false)
              }}
            >
              GitHub
            </a>
            <DarkModeToggle />
          </div>
        </div>
      </nav>

      {/* Hero Section */}
      <main id="main">
        <section className="hero">
          <div className="hero-container">
            <div className="hero-content">
            <h1 className="hero-title">
              Stop context switching.
              <span className="gradient-text"> Manage tasks from your terminal.</span>
            </h1>
            <p className="hero-subtitle">
              TaskWing is the AI-powered CLI task manager built for developers. No more juggling between Jira, Notion, and your terminal. 
              Track tasks, manage dependencies, and integrate with AI tools—all from the command line you already live in.
            </p>
            <div className="hero-buttons">
              <button 
                className="btn-primary"
                onClick={() => {
                  trackDownload('cli', { 
                    version: 'latest', 
                    source: 'hero_button',
                    buttonLocation: 'hero_section'
                  })
                }}
              >
                <span>🚀</span> Get Started Now
              </button>
              <button 
                className="btn-secondary"
                onClick={() => {
                  trackClick('View on GitHub', 'hero_section', 'https://github.com/josephgoksu/TaskWing')
                  window.open('https://github.com/josephgoksu/TaskWing', '_blank')
                }}
              >
                <span>⭐</span> View on GitHub
              </button>
            </div>
            <div className="hero-stats">
              <div className="stat">
                <span className="stat-number">12</span>
                <span className="stat-label">AI Tools</span>
              </div>
              <div className="stat">
                <span className="stat-number">0</span>
                <span className="stat-label">Context Switching</span>
              </div>
              <div className="stat">
                <span className="stat-number">100%</span>
                <span className="stat-label">Terminal Native</span>
              </div>
            </div>
            <GitHubStats />
          </div>
          <div className="hero-demo">
            <div ref={parallaxRef} className="parallax-element">
              <TypewriterTerminal 
                sequences={heroSequences}
                title="taskwing"
                className="hero-terminal"
              />
            </div>
          </div>
        </div>
        </section>

      {/* Features Section */}
      <section id="features" className="features">
        <div className="container">
          <h2 className="section-title">Built for the way developers actually work</h2>
          <div className="features-grid">
            <FeatureCard 
              icon="⚡" 
              title="Terminal Native" 
              description="No more browser tabs or heavy apps. Manage tasks directly from your terminal with blazing-fast Go performance." 
            />
            <FeatureCard 
              icon="🤖" 
              title="AI Integration" 
              description="12 MCP tools let AI assistants read, create, and manage your tasks. Break down features, generate subtasks, and get intelligent suggestions." 
            />
            <FeatureCard 
              icon="🔗" 
              title="Smart Dependencies" 
              description="Track blockers and dependencies automatically. Prevent circular dependencies and see your critical path clearly." 
            />
            <FeatureCard 
              icon="🏗️" 
              title="Project Aware" 
              description="Works with your existing workflow. Project-based configuration, Git integration, and support for multiple task formats." 
            />
            <FeatureCard 
              icon="📋" 
              title="Rich Task Data" 
              description="More than just titles. Track priorities, status, acceptance criteria, and subtasks with full metadata support." 
            />
            <FeatureCard 
              icon="🚀" 
              title="Zero Context Switch" 
              description="Stay in your flow. Create, update, and complete tasks without leaving your development environment." 
            />
          </div>
        </div>
      </section>


      {/* CLI Demo Section */}
      <section className="cli-demo">
        <div className="container">
          <h2 className="section-title">Simple commands, powerful results</h2>
          <div className="demo-grid">
            <div className="demo-content">
              <h3>Common Developer Workflows</h3>
              <ul className="command-list">
                <li><code>taskwing add "Fix auth bug"</code> - Quick task creation</li>
                <li><code>taskwing list --priority high</code> - Filter by priority</li>
                <li><code>taskwing current set abc-1</code> - Track what you're working on</li>
                <li><code>taskwing done abc-1</code> - Mark completed and move on</li>
                <li><code>taskwing mcp</code> - Let AI help plan and break down work</li>
              </ul>
              <div className="pro-tip">
                <strong>💡 Pro tip:</strong> Start the MCP server and let Claude Code automatically manage your tasks as you work.
              </div>
            </div>
            <div className="demo-terminal">
              <TypewriterTerminal 
                sequences={cliDemoSequences}
                title="taskwing demo"
                className="cli-demo-terminal"
              />
            </div>
          </div>
        </div>
      </section>

      {/* Community Section */}
      <CommunitySection />

      {/* Getting Started Section */}
      <section id="getting-started" className="getting-started">
          <div className="container">
            <h2 className="section-title">Ship faster. Start now.</h2>
            <div className="installation-content">
              <InstallationWizard />
            </div>
          </div>
        </section>

        {/* FAQ Section */}
        <FAQSection />
      </main>

      {/* Footer */}
      <footer className="footer" role="contentinfo">
        <div className="container">
          <div className="footer-content">
            <div className="footer-brand">
              <span className="logo-icon" aria-hidden="true">🪶</span>
              <span className="logo-text">TaskWing</span>
              <p>AI-assisted task management for developers</p>
            </div>
            <div className="footer-links">
              <div className="link-group">
                <h4>Product</h4>
                <a 
                  href="#features"
                  onClick={() => trackClick('Features', 'footer')}
                >
                  Features
                </a>
                <a 
                  href="https://github.com/josephgoksu/TaskWing/wiki"
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={() => trackClick('Documentation', 'footer', 'https://github.com/josephgoksu/TaskWing/wiki')}
                >
                  Documentation
                </a>
                <a 
                  href="#getting-started"
                  onClick={() => trackClick('Getting Started', 'footer')}
                >
                  Getting Started
                </a>
                <a 
                  href="#faq"
                  onClick={() => trackClick('FAQ', 'footer')}
                >
                  FAQ
                </a>
              </div>
              <div className="link-group">
                <h4>Development</h4>
                <a 
                  href="https://github.com/josephgoksu/TaskWing" 
                  target="_blank"
                  onClick={() => trackClick('GitHub', 'footer', 'https://github.com/josephgoksu/TaskWing')}
                >
                  GitHub
                </a>
                <a 
                  href="https://github.com/josephgoksu/TaskWing/issues" 
                  target="_blank"
                  onClick={() => trackClick('Issues', 'footer', 'https://github.com/josephgoksu/TaskWing/issues')}
                >
                  Issues
                </a>
                <a 
                  href="https://github.com/josephgoksu/TaskWing/blob/main/CONTRIBUTING.md" 
                  target="_blank"
                  onClick={() => trackClick('Contributing', 'footer')}
                >
                  Contributing
                </a>
              </div>
            </div>
          </div>
          <div className="footer-bottom">
            <p>&copy; 2024 TaskWing. Open source software built with <span aria-label="love">❤️</span></p>
            <p className="creator-credit">
              Created by{' '}
              <a 
                href="https://josephgoksu.com/" 
                target="_blank" 
                rel="noopener noreferrer"
                className="creator-link"
                onClick={() => trackClick('Joseph Goksu', 'footer', 'https://josephgoksu.com/')}
              >
                Joseph Goksu
              </a>
            </p>
          </div>
        </div>
      </footer>
      
      {/* Cookie Consent */}
      <CookieConsent />
    </div>
  )
}

export default App