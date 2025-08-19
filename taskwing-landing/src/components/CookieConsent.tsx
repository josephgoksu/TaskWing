import { useState, useEffect } from 'react'
import { analytics } from '../utils/analytics'
import './CookieConsent.css'

export function CookieConsent() {
  const [isVisible, setIsVisible] = useState(false)
  const [showDetails, setShowDetails] = useState(false)

  useEffect(() => {
    // Check if user has already made a choice
    const consent = localStorage.getItem('cookie_consent')
    if (!consent) {
      setIsVisible(true)
    } else {
      // Apply stored consent preference
      const consentData = JSON.parse(consent)
      analytics.setConsent(consentData.analytics)
    }
  }, [])

  const handleAcceptAll = () => {
    const consent = {
      analytics: true,
      marketing: true,
      functional: true,
      timestamp: Date.now()
    }
    
    localStorage.setItem('cookie_consent', JSON.stringify(consent))
    analytics.setConsent(true)
    setIsVisible(false)
  }

  const handleRejectAll = () => {
    const consent = {
      analytics: false,
      marketing: false,
      functional: true, // Essential cookies always allowed
      timestamp: Date.now()
    }
    
    localStorage.setItem('cookie_consent', JSON.stringify(consent))
    analytics.setConsent(false)
    setIsVisible(false)
  }

  const handleCustomize = () => {
    setShowDetails(!showDetails)
  }

  const handleSavePreferences = (preferences: Record<string, boolean>) => {
    const consent = {
      ...preferences,
      functional: true, // Essential cookies always required
      timestamp: Date.now()
    }
    
    localStorage.setItem('cookie_consent', JSON.stringify(consent))
    analytics.setConsent(preferences.analytics || false)
    setIsVisible(false)
  }

  if (!isVisible) return null

  return (
    <div className="cookie-consent">
      <div className="cookie-consent-content">
        <div className="cookie-consent-main">
          <h3>🍪 We use cookies</h3>
          <p>
            We use cookies to enhance your experience, analyze site traffic, and for marketing purposes. 
            By clicking "Accept All", you consent to our use of cookies.
          </p>
          
          {showDetails && (
            <CookieDetails onSave={handleSavePreferences} />
          )}
          
          <div className="cookie-consent-actions">
            <button 
              className="btn btn-secondary"
              onClick={handleRejectAll}
            >
              Reject All
            </button>
            <button 
              className="btn btn-outline"
              onClick={handleCustomize}
            >
              {showDetails ? 'Hide Details' : 'Customize'}
            </button>
            <button 
              className="btn btn-primary"
              onClick={handleAcceptAll}
            >
              Accept All
            </button>
          </div>
        </div>
        
        <div className="cookie-consent-links">
          <a href="/privacy" target="_blank" rel="noopener noreferrer">
            Privacy Policy
          </a>
          <a href="/cookies" target="_blank" rel="noopener noreferrer">
            Cookie Policy
          </a>
        </div>
      </div>
    </div>
  )
}

interface CookieDetailsProps {
  onSave: (preferences: Record<string, boolean>) => void
}

function CookieDetails({ onSave }: CookieDetailsProps) {
  const [preferences, setPreferences] = useState({
    analytics: false,
    marketing: false,
    functional: true
  })

  const handleToggle = (type: keyof typeof preferences) => {
    if (type === 'functional') return // Can't disable essential cookies
    
    setPreferences(prev => ({
      ...prev,
      [type]: !prev[type]
    }))
  }

  const handleSave = () => {
    onSave(preferences)
  }

  return (
    <div className="cookie-details">
      <h4>Cookie Preferences</h4>
      
      <div className="cookie-category">
        <div className="cookie-category-header">
          <label>
            <input
              type="checkbox"
              checked={preferences.functional}
              disabled
              onChange={() => {}}
            />
            <span className="checkbox-label">
              <strong>Essential Cookies</strong>
              <span className="required">Required</span>
            </span>
          </label>
        </div>
        <p>These cookies are necessary for the website to function and cannot be disabled.</p>
      </div>

      <div className="cookie-category">
        <div className="cookie-category-header">
          <label>
            <input
              type="checkbox"
              checked={preferences.analytics}
              onChange={() => handleToggle('analytics')}
            />
            <span className="checkbox-label">
              <strong>Analytics Cookies</strong>
            </span>
          </label>
        </div>
        <p>These cookies help us understand how visitors interact with our website by collecting and reporting information anonymously.</p>
      </div>

      <div className="cookie-category">
        <div className="cookie-category-header">
          <label>
            <input
              type="checkbox"
              checked={preferences.marketing}
              onChange={() => handleToggle('marketing')}
            />
            <span className="checkbox-label">
              <strong>Marketing Cookies</strong>
            </span>
          </label>
        </div>
        <p>These cookies are used to track visitors across websites for marketing and advertising purposes.</p>
      </div>

      <button className="btn btn-primary save-preferences" onClick={handleSave}>
        Save Preferences
      </button>
    </div>
  )
}