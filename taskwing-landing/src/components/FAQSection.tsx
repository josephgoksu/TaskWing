import { useState, useMemo } from 'react'
import './FAQSection.css'

interface FAQ {
  id: string
  question: string
  answer: string
  category: string
  keywords: string[]
}

interface FAQCategory {
  id: string
  name: string
  icon: string
  description: string
}

const categories: FAQCategory[] = [
  {
    id: 'installation',
    name: 'Installation',
    icon: '⚙️',
    description: 'Getting TaskWing installed and set up'
  },
  {
    id: 'usage',
    name: 'Usage',
    icon: '🚀',
    description: 'How to use TaskWing effectively'
  },
  {
    id: 'mcp',
    name: 'MCP Integration',
    icon: '🤖',
    description: 'Model Context Protocol features'
  },
  {
    id: 'troubleshooting',
    name: 'Troubleshooting',
    icon: '🔧',
    description: 'Common issues and solutions'
  },
  {
    id: 'advanced',
    name: 'Advanced',
    icon: '⚡',
    description: 'Advanced features and configuration'
  }
]

const faqs: FAQ[] = [
  {
    id: '1',
    question: 'What is TaskWing and how is it different from other task managers?',
    answer: 'TaskWing is a CLI-native task manager specifically designed for developers, with built-in AI integration through Model Context Protocol (MCP). Unlike traditional task managers, TaskWing integrates directly into your development workflow, supports complex dependency tracking, and allows AI tools to interact with your tasks for intelligent assistance with planning, breakdown, and management.',
    category: 'usage',
    keywords: ['taskwing', 'CLI', 'developers', 'AI', 'MCP', 'workflow']
  },
  {
    id: '2',
    question: 'How do I install TaskWing?',
    answer: 'TaskWing can be installed in several ways:\n\n**Go Install (Recommended):**\n```\ngo install github.com/josephgoksu/TaskWing@latest\n```\n\n**Homebrew:**\n```\nbrew tap taskwing/taskwing\nbrew install taskwing\n```\n\n**Binary Download:**\nDownload pre-built binaries from our [GitHub releases](https://github.com/josephgoksu/TaskWing/releases)\n\n**Build from Source:**\n```\ngit clone https://github.com/josephgoksu/TaskWing.git\ncd TaskWing\ngo build -o taskwing main.go\n```',
    category: 'installation',
    keywords: ['install', 'go', 'homebrew', 'binary', 'build', 'setup']
  },
  {
    id: '3',
    question: 'What is Model Context Protocol (MCP) and why should I care?',
    answer: 'Model Context Protocol (MCP) is a standardized way for AI tools to interact with external systems and data. TaskWing implements MCP with 12 specialized tools that enable AI assistants to:\n\n• Create and manage tasks intelligently\n• Break down complex projects into manageable tasks\n• Analyze task dependencies and suggest optimizations\n• Provide context-aware project insights\n• Automate routine task management operations\n\nThis means your AI tools can understand your project state and help with planning and execution.',
    category: 'mcp',
    keywords: ['MCP', 'AI', 'protocol', 'tools', 'automation', 'planning']
  },
  {
    id: '4',
    question: 'How do I get started after installation?',
    answer: 'After installing TaskWing, follow these steps:\n\n1. **Initialize your project:**\n   ```\n   cd your-project\n   taskwing init\n   ```\n\n2. **Create your first task:**\n   ```\n   taskwing add "Set up project structure"\n   ```\n\n3. **List your tasks:**\n   ```\n   taskwing list\n   ```\n\n4. **Start the MCP server for AI integration:**\n   ```\n   taskwing mcp\n   ```\n\n5. **Mark tasks as complete:**\n   ```\n   taskwing done <task-id>\n   ```',
    category: 'usage',
    keywords: ['getting started', 'init', 'first task', 'commands', 'workflow']
  },
  {
    id: '5',
    question: 'Can I use TaskWing with my existing project management tools?',
    answer: 'Yes! TaskWing is designed to complement your existing tools rather than replace them. You can:\n\n• Export tasks to various formats (JSON, CSV, Markdown)\n• Use TaskWing for local development task tracking while syncing higher-level planning with tools like Jira or Linear\n• Integrate TaskWing data with CI/CD pipelines\n• Use the MCP integration to have AI tools analyze your TaskWing data alongside other project information\n\nTaskWing excels at developer-focused, granular task management that larger tools often miss.',
    category: 'usage',
    keywords: ['integration', 'export', 'existing tools', 'jira', 'linear', 'workflow']
  },
  {
    id: '6',
    question: 'How does dependency tracking work?',
    answer: 'TaskWing includes intelligent dependency tracking with automatic circular dependency detection:\n\n• **Set dependencies:** Tasks can depend on one or more other tasks\n• **Automatic validation:** Prevents circular dependencies that would create deadlocks\n• **Visual indicators:** See dependency chains and blocking relationships\n• **Smart ordering:** List and filter tasks based on dependency order\n• **Dependency completion:** Track which dependencies are holding up progress\n\nExample:\n```\ntaskwing add "Deploy to production" --depends-on "run-tests,code-review"\n```',
    category: 'advanced',
    keywords: ['dependencies', 'blocking', 'circular', 'validation', 'tracking']
  },
  {
    id: '7',
    question: 'What should I do if TaskWing is not working or I encounter errors?',
    answer: 'Here are common troubleshooting steps:\n\n**1. Check installation:**\n```\ntaskwing --version\n```\n\n**2. Verify Go version (1.19+ required):**\n```\ngo version\n```\n\n**3. Clear corrupted data:**\n```\nrm -rf .taskwing\ntaskwing init\n```\n\n**4. Check permissions:**\nEnsure TaskWing can write to your project directory\n\n**5. Update to latest version:**\n```\ngo install github.com/josephgoksu/TaskWing@latest\n```\n\n**Still having issues?** Open an issue on [GitHub](https://github.com/josephgoksu/TaskWing/issues) with:\n• Your OS and Go version\n• TaskWing version\n• Complete error message\n• Steps to reproduce',
    category: 'troubleshooting',
    keywords: ['errors', 'troubleshooting', 'not working', 'fix', 'debug', 'issues']
  },
  {
    id: '8',
    question: 'How do I configure TaskWing for my team or project?',
    answer: 'TaskWing supports flexible configuration at multiple levels:\n\n**Project-level (.taskwing/.taskwing.yaml):**\n```yaml\nproject:\n  name: "My Project"\n  rootDir: ".taskwing"\ndata:\n  format: "json"  # or yaml, toml\n```\n\n**User-level (~/.taskwing.yaml):**\nSet personal defaults for all projects\n\n**Environment variables:**\n```\nTASKWING_PROJECT_NAME="My Project"\nTASKWING_DATA_FORMAT="yaml"\n```\n\n**Team sharing:** Commit `.taskwing.yaml` to version control for team-wide settings while keeping task data local or shared as needed.',
    category: 'advanced',
    keywords: ['configuration', 'team', 'yaml', 'environment', 'settings', 'customize']
  },
  {
    id: '9',
    question: 'Can I use TaskWing with AI assistants like Claude or ChatGPT?',
    answer: 'Absolutely! TaskWing is designed for AI integration:\n\n**MCP Server Mode:**\n```\ntaskwing mcp\n```\nThis starts a server that AI tools can connect to for direct task manipulation.\n\n**Supported AI Tools:**\n• Claude (with MCP support)\n• Any MCP-compatible AI assistant\n• Custom integrations via the MCP protocol\n\n**AI Capabilities:**\n• Task creation and breakdown\n• Project analysis and insights\n• Dependency optimization\n• Progress tracking and reporting\n• Automated task management\n\nThe AI can read your current tasks, understand project context, and help with planning and execution.',
    category: 'mcp',
    keywords: ['AI', 'Claude', 'ChatGPT', 'assistant', 'MCP server', 'integration']
  },
  {
    id: '10',
    question: 'Is TaskWing free? What\'s the licensing?',
    answer: 'TaskWing is **completely free and open source** under the MIT License. This means:\n\n• ✅ **Free for personal use**\n• ✅ **Free for commercial use**\n• ✅ **No subscription fees**\n• ✅ **No usage limits**\n• ✅ **Full source code available**\n• ✅ **Can be modified and redistributed**\n\n**Support the project:**\n• ⭐ Star us on [GitHub](https://github.com/josephgoksu/TaskWing)\n• 🐛 Report bugs and request features\n• 🤝 Contribute code or documentation\n• 💬 Help other users in discussions\n\nWe believe great developer tools should be accessible to everyone.',
    category: 'usage',
    keywords: ['free', 'open source', 'MIT', 'license', 'cost', 'pricing', 'commercial']
  }
]

export function FAQSection() {
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedCategory, setSelectedCategory] = useState('all')
  const [expandedFAQs, setExpandedFAQs] = useState<Set<string>>(new Set())

  // Filter FAQs based on search and category
  const filteredFAQs = useMemo(() => {
    return faqs.filter(faq => {
      const matchesCategory = selectedCategory === 'all' || faq.category === selectedCategory
      const matchesSearch = searchTerm === '' || 
        faq.question.toLowerCase().includes(searchTerm.toLowerCase()) ||
        faq.answer.toLowerCase().includes(searchTerm.toLowerCase()) ||
        faq.keywords.some(keyword => keyword.toLowerCase().includes(searchTerm.toLowerCase()))
      
      return matchesCategory && matchesSearch
    })
  }, [searchTerm, selectedCategory])

  const toggleFAQ = (id: string) => {
    const newExpanded = new Set(expandedFAQs)
    if (newExpanded.has(id)) {
      newExpanded.delete(id)
    } else {
      newExpanded.add(id)
    }
    setExpandedFAQs(newExpanded)
  }

  const expandAll = () => {
    setExpandedFAQs(new Set(filteredFAQs.map(faq => faq.id)))
  }

  const collapseAll = () => {
    setExpandedFAQs(new Set())
  }

  const formatAnswer = (answer: string) => {
    // Convert markdown-like formatting to HTML
    return answer
      .split('\n')
      .map((line, index) => {
        // Handle code blocks
        if (line.startsWith('```')) {
          return null // Skip markdown code block delimiters
        }
        
        // Handle bold text
        line = line.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        
        // Handle code inline
        line = line.replace(/`([^`]+)`/g, '<code>$1</code>')
        
        // Handle bullet points
        if (line.startsWith('•')) {
          return <li key={index} dangerouslySetInnerHTML={{ __html: line.substring(1).trim() }} />
        }
        
        // Handle empty lines
        if (line.trim() === '') {
          return <br key={index} />
        }
        
        // Regular paragraphs
        return <p key={index} dangerouslySetInnerHTML={{ __html: line }} />
      })
      .filter(Boolean)
  }

  return (
    <section className="faq-section" id="faq">
      <div className="container">
        <div className="faq-header">
          <h2 className="section-title">Frequently Asked Questions</h2>
          <p className="faq-subtitle">
            Everything you need to know about TaskWing. Can't find what you're looking for? 
            <a href="https://github.com/josephgoksu/TaskWing/discussions" target="_blank" rel="noopener noreferrer">
              Ask in our community discussions
            </a>.
          </p>
        </div>

        {/* Search and Controls */}
        <div className="faq-controls">
          <div className="search-box">
            <div className="search-input-wrapper">
              <span className="search-icon">🔍</span>
              <input
                type="text"
                placeholder="Search FAQs..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="search-input"
              />
              {searchTerm && (
                <button 
                  className="clear-search"
                  onClick={() => setSearchTerm('')}
                  aria-label="Clear search"
                >
                  ✕
                </button>
              )}
            </div>
          </div>
          
          <div className="expand-controls">
            <button className="expand-btn" onClick={expandAll}>
              Expand All
            </button>
            <button className="expand-btn" onClick={collapseAll}>
              Collapse All
            </button>
          </div>
        </div>

        {/* Category Filter */}
        <div className="category-filters">
          <button
            className={`category-filter ${selectedCategory === 'all' ? 'active' : ''}`}
            onClick={() => setSelectedCategory('all')}
          >
            <span className="category-icon">📋</span>
            <span className="category-name">All</span>
          </button>
          
          {categories.map(category => (
            <button
              key={category.id}
              className={`category-filter ${selectedCategory === category.id ? 'active' : ''}`}
              onClick={() => setSelectedCategory(category.id)}
              title={category.description}
            >
              <span className="category-icon">{category.icon}</span>
              <span className="category-name">{category.name}</span>
            </button>
          ))}
        </div>

        {/* FAQ Results Info */}
        <div className="faq-results-info">
          <span className="results-count">
            {filteredFAQs.length} {filteredFAQs.length === 1 ? 'question' : 'questions'}
            {searchTerm && ` matching "${searchTerm}"`}
            {selectedCategory !== 'all' && ` in ${categories.find(c => c.id === selectedCategory)?.name}`}
          </span>
        </div>

        {/* FAQ List */}
        <div className="faq-list">
          {filteredFAQs.length === 0 ? (
            <div className="no-results">
              <span className="no-results-icon">🤔</span>
              <h3>No questions found</h3>
              <p>
                Try adjusting your search terms or 
                <button 
                  className="reset-filters"
                  onClick={() => {
                    setSearchTerm('')
                    setSelectedCategory('all')
                  }}
                >
                  reset filters
                </button>
              </p>
            </div>
          ) : (
            filteredFAQs.map(faq => (
              <div key={faq.id} className={`faq-item ${expandedFAQs.has(faq.id) ? 'expanded' : ''}`}>
                <button
                  className="faq-question"
                  onClick={() => toggleFAQ(faq.id)}
                  aria-expanded={expandedFAQs.has(faq.id)}
                  aria-controls={`faq-answer-${faq.id}`}
                >
                  <span className="question-text">{faq.question}</span>
                  <span className="expand-icon">
                    {expandedFAQs.has(faq.id) ? '−' : '+'}
                  </span>
                </button>
                
                <div 
                  id={`faq-answer-${faq.id}`}
                  className="faq-answer"
                  aria-hidden={!expandedFAQs.has(faq.id)}
                >
                  <div className="answer-content">
                    {formatAnswer(faq.answer)}
                  </div>
                </div>
              </div>
            ))
          )}
        </div>

        {/* Help Section */}
        <div className="faq-help">
          <h3>Still need help?</h3>
          <div className="help-options">
            <a 
              href="https://github.com/josephgoksu/TaskWing/discussions" 
              target="_blank" 
              rel="noopener noreferrer"
              className="help-option"
            >
              <span className="help-icon">💬</span>
              <span className="help-text">Community Discussions</span>
            </a>
            <a 
              href="https://github.com/josephgoksu/TaskWing/issues" 
              target="_blank" 
              rel="noopener noreferrer"
              className="help-option"
            >
              <span className="help-icon">🐛</span>
              <span className="help-text">Report an Issue</span>
            </a>
            <a 
              href="https://github.com/josephgoksu/TaskWing#documentation" 
              target="_blank" 
              rel="noopener noreferrer"
              className="help-option"
            >
              <span className="help-icon">📚</span>
              <span className="help-text">Documentation</span>
            </a>
          </div>
        </div>
      </div>
    </section>
  )
}