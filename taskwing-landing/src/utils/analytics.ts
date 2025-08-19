// Google Analytics 4 Implementation
declare global {
  interface Window {
    gtag: (...args: unknown[]) => void;
    dataLayer: unknown[];
  }
}

export interface AnalyticsEvent {
  action: string;
  category: string;
  label?: string;
  value?: number;
  custom_parameters?: Record<string, unknown>;
}

export interface ConversionEvent {
  event_name: string;
  currency?: string;
  value?: number;
  transaction_id?: string;
  items?: unknown[];
  custom_parameters?: Record<string, unknown>;
}

class Analytics {
  private isInitialized = false;
  private gaId: string | null = null;

  constructor() {
    // Initialize data layer
    window.dataLayer = window.dataLayer || [];
    window.gtag = function(...args) {
      window.dataLayer.push(args);
    };
  }

  // Initialize GA4
  init(measurementId: string, config: Record<string, unknown> = {}) {
    if (this.isInitialized) return;
    
    this.gaId = measurementId;
    
    // Load GA4 script
    const script = document.createElement('script');
    script.async = true;
    script.src = `https://www.googletagmanager.com/gtag/js?id=${measurementId}`;
    document.head.appendChild(script);

    // Configure GA4
    window.gtag('js', new Date());
    window.gtag('config', measurementId, {
      // Enhanced ecommerce
      send_page_view: true,
      allow_google_signals: true,
      allow_ad_personalization_signals: true,
      // Privacy settings
      anonymize_ip: true,
      cookie_flags: 'SameSite=Strict;Secure',
      ...config
    });

    this.isInitialized = true;
    console.log('Analytics initialized with GA4:', measurementId);
  }

  // Track page views
  trackPageView(page_title?: string, page_location?: string) {
    if (!this.isInitialized) return;
    
    window.gtag('event', 'page_view', {
      page_title: page_title || document.title,
      page_location: page_location || window.location.href,
    });
  }

  // Track custom events
  trackEvent(event: AnalyticsEvent) {
    if (!this.isInitialized) return;
    
    window.gtag('event', event.action, {
      event_category: event.category,
      event_label: event.label,
      value: event.value,
      ...event.custom_parameters
    });
  }

  // Track conversions (downloads, signups, etc.)
  trackConversion(event: ConversionEvent) {
    if (!this.isInitialized) return;
    
    window.gtag('event', event.event_name, {
      currency: event.currency || 'USD',
      value: event.value || 0,
      transaction_id: event.transaction_id,
      items: event.items || [],
      ...event.custom_parameters
    });
  }

  // Track download events
  trackDownload(downloadType: string, version?: string, source?: string) {
    this.trackConversion({
      event_name: 'download',
      value: 1,
      custom_parameters: {
        download_type: downloadType,
        version: version || 'latest',
        source: source || 'website',
        content_group1: 'download'
      }
    });
  }

  // Track button clicks
  trackButtonClick(buttonName: string, location: string, url?: string) {
    this.trackEvent({
      action: 'click',
      category: 'button',
      label: `${buttonName} - ${location}`,
      custom_parameters: {
        button_name: buttonName,
        button_location: location,
        destination_url: url
      }
    });
  }

  // Track scroll depth
  trackScrollDepth(percentage: number) {
    this.trackEvent({
      action: 'scroll',
      category: 'engagement',
      label: `${percentage}%`,
      value: percentage,
      custom_parameters: {
        scroll_depth: percentage
      }
    });
  }

  // Track user engagement
  trackEngagement(event_name: string, engagement_time_msec: number) {
    if (!this.isInitialized) return;
    
    window.gtag('event', 'user_engagement', {
      engagement_time_msec,
      custom_parameters: {
        event_name,
        page_title: document.title,
        page_location: window.location.href
      }
    });
  }

  // Track A/B test variants
  trackAbTest(experiment_id: string, variant_id: string) {
    if (!this.isInitialized) return;
    
    window.gtag('config', this.gaId!, {
      custom_map: {
        custom_parameter_1: 'experiment_id',
        custom_parameter_2: 'variant_id'
      }
    });

    window.gtag('event', 'ab_test_impression', {
      experiment_id,
      variant_id,
      custom_parameter_1: experiment_id,
      custom_parameter_2: variant_id
    });
  }

  // Enable/disable analytics based on consent
  setConsent(granted: boolean) {
    window.gtag('consent', 'update', {
      analytics_storage: granted ? 'granted' : 'denied',
      ad_storage: granted ? 'granted' : 'denied',
      ad_user_data: granted ? 'granted' : 'denied',
      ad_personalization: granted ? 'granted' : 'denied'
    });
  }
}

// Create singleton instance
export const analytics = new Analytics();

// Scroll depth tracking utility
export class ScrollTracker {
  private thresholds = [25, 50, 75, 90, 100];
  private triggered = new Set<number>();

  constructor() {
    this.setupScrollTracking();
  }

  private setupScrollTracking() {
    let ticking = false;

    const trackScroll = () => {
      const scrollTop = window.pageYOffset;
      const docHeight = document.documentElement.scrollHeight - window.innerHeight;
      const scrollPercent = Math.round((scrollTop / docHeight) * 100);

      this.thresholds.forEach(threshold => {
        if (scrollPercent >= threshold && !this.triggered.has(threshold)) {
          this.triggered.add(threshold);
          analytics.trackScrollDepth(threshold);
        }
      });

      ticking = false;
    };

    const requestTick = () => {
      if (!ticking) {
        requestAnimationFrame(trackScroll);
        ticking = true;
      }
    };

    window.addEventListener('scroll', requestTick, { passive: true });
  }
}

// A/B Testing Framework
export class ABTestFramework {
  private experiments: Map<string, string> = new Map();

  // Define an A/B test
  defineExperiment(
    experimentId: string,
    variants: string[],
    trafficAllocation: number = 1.0
  ) {
    const variant = this.assignVariant(experimentId, variants, trafficAllocation);
    this.experiments.set(experimentId, variant);
    
    // Track the variant assignment
    analytics.trackAbTest(experimentId, variant);
    
    return variant;
  }

  // Get variant for user
  private assignVariant(
    experimentId: string,
    variants: string[],
    trafficAllocation: number
  ): string {
    // Use localStorage for consistent variant assignment
    const storageKey = `ab_test_${experimentId}`;
    const stored = localStorage.getItem(storageKey);
    
    if (stored && variants.includes(stored)) {
      return stored;
    }

    // Determine if user should be in experiment
    const userId = this.getUserId();
    const hash = this.hashString(`${userId}_${experimentId}`);
    const shouldInclude = (hash % 100) < (trafficAllocation * 100);
    
    if (!shouldInclude) {
      return 'control';
    }

    // Assign variant
    const variantIndex = hash % variants.length;
    const variant = variants[variantIndex];
    
    localStorage.setItem(storageKey, variant);
    return variant;
  }

  // Get consistent user ID
  private getUserId(): string {
    let userId = localStorage.getItem('user_id');
    if (!userId) {
      userId = Math.random().toString(36).substring(2, 15);
      localStorage.setItem('user_id', userId);
    }
    return userId;
  }

  // Simple hash function
  private hashString(str: string): number {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i);
      hash = ((hash << 5) - hash) + char;
      hash = hash & hash; // Convert to 32-bit integer
    }
    return Math.abs(hash);
  }

  // Get variant for an experiment
  public getVariant(experimentId: string): string | null {
    return this.experiments.get(experimentId) || null;
  }
}

export const abTesting = new ABTestFramework();