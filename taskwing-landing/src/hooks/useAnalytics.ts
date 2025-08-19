import { useEffect, useCallback } from 'react'
import { analytics, ScrollTracker, abTesting } from '../utils/analytics'

// Hook for initializing analytics
export function useAnalytics(measurementId?: string) {
  useEffect(() => {
    if (measurementId) {
      analytics.init(measurementId, {
        // Additional GA4 configuration
        send_page_view: true,
        cookie_domain: 'auto',
        cookie_expires: 63072000, // 2 years
        allow_google_signals: true,
        allow_ad_personalization_signals: true
      })

      // Initialize scroll tracking
      new ScrollTracker()
    }
  }, [measurementId])

  return analytics
}

// Hook for tracking download events
export function useDownloadTracking() {
  const trackDownload = useCallback((downloadType: string, options?: {
    version?: string
    source?: string
    buttonLocation?: string
  }) => {
    analytics.trackDownload(downloadType, options?.version, options?.source)
    
    // Also track as button click
    if (options?.buttonLocation) {
      analytics.trackButtonClick(
        `Download ${downloadType}`,
        options.buttonLocation,
        undefined
      )
    }
  }, [])

  return { trackDownload }
}

// Hook for tracking button clicks
export function useButtonTracking() {
  const trackClick = useCallback((
    buttonName: string, 
    location: string, 
    url?: string
  ) => {
    analytics.trackButtonClick(buttonName, location, url)
  }, [])

  return { trackClick }
}

// Hook for A/B testing
export function useAbTesting() {
  const createExperiment = useCallback((
    experimentId: string,
    variants: string[],
    trafficAllocation: number = 1.0
  ) => {
    return abTesting.defineExperiment(experimentId, variants, trafficAllocation)
  }, [])

  const getVariant = useCallback((experimentId: string) => {
    return abTesting.getVariant(experimentId)
  }, [])

  return { createExperiment, getVariant }
}

// Hook for engagement tracking
export function useEngagementTracking() {
  useEffect(() => {
    let startTime = Date.now()
    let isActive = true
    let totalEngagementTime = 0

    const trackEngagement = () => {
      if (isActive) {
        const sessionTime = Date.now() - startTime
        totalEngagementTime += sessionTime
        analytics.trackEngagement('page_engagement', totalEngagementTime)
      }
    }

    const handleVisibilityChange = () => {
      if (document.hidden) {
        if (isActive) {
          totalEngagementTime += Date.now() - startTime
          isActive = false
        }
      } else {
        startTime = Date.now()
        isActive = true
      }
    }

    const handleBeforeUnload = () => {
      trackEngagement()
    }

    // Track engagement every 30 seconds
    const engagementInterval = setInterval(trackEngagement, 30000)

    // Listen for page visibility changes
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('beforeunload', handleBeforeUnload)

    return () => {
      clearInterval(engagementInterval)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      window.removeEventListener('beforeunload', handleBeforeUnload)
      trackEngagement() // Final engagement tracking
    }
  }, [])
}

// Hook for form tracking
export function useFormTracking() {
  const trackFormStart = useCallback((formName: string) => {
    analytics.trackEvent({
      action: 'form_start',
      category: 'form',
      label: formName
    })
  }, [])

  const trackFormComplete = useCallback((formName: string) => {
    analytics.trackEvent({
      action: 'form_complete',
      category: 'form',
      label: formName
    })
  }, [])

  const trackFormError = useCallback((formName: string, errorType: string) => {
    analytics.trackEvent({
      action: 'form_error',
      category: 'form',
      label: `${formName} - ${errorType}`
    })
  }, [])

  return { trackFormStart, trackFormComplete, trackFormError }
}