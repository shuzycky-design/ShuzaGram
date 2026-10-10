import lottie from "lottie-web/build/player/lottie_light_canvas";
import { useEffect, useRef, useState } from "react";

// StaticLottie renders a single (first) frame of a Lottie/TGS animation instead
// of looping it, so a grid of many stickers/emoji does not keep the canvas
// rendering and pinning the CPU. It plays only while hovered, then resets to the
// static frame. Use it for list/grid previews; keep the looping player for
// single, focused previews.
//
// lazy=true additionally defers calling `loader()` (the network fetch) until
// the element has actually scrolled into the viewport, via IntersectionObserver
// -- for a long list this turns "every row fetches its animation on mount"
// into "only the rows the user actually scrolls to ever fetch anything".
export function StaticLottie({
  loader,
  cacheKey,
  className,
  playOnHover = true,
  lazy = false,
  onError
}: {
  loader: () => Promise<Record<string, unknown>>;
  cacheKey: string;
  className?: string;
  playOnHover?: boolean;
  lazy?: boolean;
  onError?: () => void;
}) {
  const host = useRef<HTMLDivElement>(null);
  const animation = useRef<ReturnType<typeof lottie.loadAnimation> | null>(null);
  const [visible, setVisible] = useState(!lazy);

  useEffect(() => {
    if (!lazy || visible || !host.current) return;
    const el = host.current;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "200px" }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [lazy, visible]);

  useEffect(() => {
    if (!visible) return;
    let cancelled = false;
    loader()
      .then((data) => {
        if (cancelled || !host.current) return;
        animation.current?.destroy();
        animation.current = lottie.loadAnimation({
          container: host.current,
          renderer: "canvas",
          loop: true,
          autoplay: false,
          animationData: structuredClone(data)
        });
        animation.current.goToAndStop(0, true);
      })
      .catch(() => onError?.());
    return () => {
      cancelled = true;
      animation.current?.destroy();
      animation.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cacheKey, visible]);

  function play() {
    if (playOnHover) animation.current?.play();
  }
  function reset() {
    if (playOnHover) animation.current?.goToAndStop(0, true);
  }

  return <div className={className} ref={host} onMouseEnter={play} onMouseLeave={reset} />;
}
