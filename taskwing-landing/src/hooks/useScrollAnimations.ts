import { useEffect, useRef } from "react";

interface ScrollAnimationOptions {
  threshold?: number;
  rootMargin?: string;
  triggerOnce?: boolean;
}

export function useScrollAnimation(options: ScrollAnimationOptions = {}) {
  const {
    threshold = 0.01, // Lower threshold to trigger earlier
    rootMargin = "0px", // No negative margin
    triggerOnce = true,
  } = options;

  const elementRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const element = elementRef.current;
    if (!element) return;

    // Check if user prefers reduced motion
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (prefersReducedMotion) {
      element.classList.add("animate-in");
      return;
    }

    // Check if element is already in viewport on mount
    const checkInitialVisibility = () => {
      const rect = element.getBoundingClientRect();
      const isVisible = rect.top < window.innerHeight && rect.bottom > 0;
      if (isVisible) {
        element.classList.add("animate-in");
      }
    };
    
    // Run initial check
    checkInitialVisibility();
    
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add("animate-in");
            
            if (triggerOnce) {
              observer.unobserve(entry.target);
            }
          } else if (!triggerOnce) {
            entry.target.classList.remove("animate-in");
          }
        });
      },
      {
        threshold,
        rootMargin,
      }
    );

    observer.observe(element);

    return () => {
      observer.disconnect();
    };
  }, [threshold, rootMargin, triggerOnce]);

  return elementRef;
}

export function useParallaxEffect(speed: number = 0.3) {
  const elementRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const element = elementRef.current;
    if (!element) return;

    // Check if user prefers reduced motion
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (prefersReducedMotion) {
      return;
    }

    let ticking = false;
    let lastScrollY = 0;

    const updateParallax = () => {
      const scrollY = window.pageYOffset;
      
      // Only update if scroll position changed significantly
      if (Math.abs(scrollY - lastScrollY) < 1) {
        ticking = false;
        return;
      }
      
      lastScrollY = scrollY;
      const rate = scrollY * -speed;

      if (element) {
        // Use transform3d for hardware acceleration with reduced movement
        element.style.transform = `translate3d(0, ${Math.round(rate)}px, 0)`;
      }

      ticking = false;
    };

    const requestTick = () => {
      if (!ticking) {
        requestAnimationFrame(updateParallax);
        ticking = true;
      }
    };

    // Set initial will-change
    element.style.willChange = 'transform';
    
    window.addEventListener("scroll", requestTick, { passive: true });

    return () => {
      window.removeEventListener("scroll", requestTick);
      // Reset will-change when component unmounts
      if (element) {
        element.style.willChange = 'auto';
        element.style.transform = '';
      }
    };
  }, [speed]);

  return elementRef;
}

export function useHoverAnimation() {
  const elementRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const element = elementRef.current;
    if (!element) return;

    // Check if user prefers reduced motion
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (prefersReducedMotion) {
      return;
    }

    // Check if device supports hover (not touch-only)
    const supportsHover = window.matchMedia('(hover: hover)').matches;
    if (!supportsHover) {
      return;
    }

    const handleMouseEnter = () => {
      element.classList.add("hover-animate");
    };

    const handleMouseLeave = () => {
      element.classList.remove("hover-animate");
    };

    element.addEventListener("mouseenter", handleMouseEnter, { passive: true });
    element.addEventListener("mouseleave", handleMouseLeave, { passive: true });

    return () => {
      element.removeEventListener("mouseenter", handleMouseEnter);
      element.removeEventListener("mouseleave", handleMouseLeave);
    };
  }, []);

  return elementRef;
}