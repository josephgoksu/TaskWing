import { GitHubStats } from './GitHubStats'
import './CommunitySection.css'

export function CommunitySection() {
  return (
    <section className="community-section">
      <div className="container">
        <h2 className="section-title">Join the TaskWing Community</h2>
        
        <div className="community-content">
          <p className="community-description">
            TaskWing is an open-source project that's growing. Be among the first to experience 
            AI-powered task management directly from your terminal.
          </p>
          
          <GitHubStats />
          
          <div className="community-cta">
            <h3>Ready to streamline your workflow?</h3>
            <p>TaskWing is free and open source. Try it today and let us know what you think!</p>
            
            <div className="community-actions">
              <a 
                href="https://github.com/josephgoksu/TaskWing"
                target="_blank"
                rel="noopener noreferrer"
                className="btn-primary"
              >
                ⭐ Star on GitHub
              </a>
              <a 
                href="https://github.com/josephgoksu/TaskWing/issues"
                target="_blank"
                rel="noopener noreferrer"
                className="btn-secondary"
              >
                Share Feedback
              </a>
            </div>
          </div>
          
          <div className="early-adopter">
            <h4>Be an Early Adopter</h4>
            <p>
              TaskWing is actively being developed. Your feedback helps shape the future of 
              terminal-based task management. Join our growing community of developers who 
              prefer the command line.
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}