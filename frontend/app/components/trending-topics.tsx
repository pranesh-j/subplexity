"use client"

import { useEffect, useRef } from "react"
import { ArrowUpIcon } from "lucide-react"

const trendingTopics = [
  {
    title: "r/technology",
    posts: "2.5k posts",
    type: "Trending",
  },
  {
    title: "r/programming",
    posts: "1.8k posts",
    type: "Hot",
  },
  {
    title: "r/science",
    posts: "1.2k posts",
    type: "Popular",
  },
  {
    title: "r/askreddit",
    posts: "3.1k posts",
    type: "Trending",
  },
]

export function TrendingTopics() {
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const scrollContainer = scrollRef.current
    if (!scrollContainer) return

    let animationFrameId: number
    let scrollPosition = 0

    const scroll = () => {
      scrollPosition += 0.5
      if (scrollPosition >= scrollContainer.scrollWidth - scrollContainer.clientWidth) {
        scrollPosition = 0
      }
      scrollContainer.scrollLeft = scrollPosition
      animationFrameId = requestAnimationFrame(scroll)
    }

    animationFrameId = requestAnimationFrame(scroll)

    return () => {
      cancelAnimationFrame(animationFrameId)
    }
  }, [])

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 text-zinc-400">
        <ArrowUpIcon className="h-4 w-4" />
        <span className="text-sm font-medium">Trending on Reddit</span>
      </div>
      <div ref={scrollRef} className="flex space-x-4 overflow-x-hidden py-2">
        {[...trendingTopics, ...trendingTopics].map((topic, i) => (
          <button
            key={`${topic.title}-${i}`}
            className="flex items-center gap-3 px-4 py-2 rounded-lg bg-zinc-900/50 hover:bg-zinc-800/50 transition-colors group"
          >
            <div className="w-8 h-8 rounded-full bg-[#FF4500]/10 flex items-center justify-center text-[#FF4500]">
              r/
            </div>
            <div className="text-left">
              <p className="text-sm text-zinc-300 group-hover:text-white transition-colors font-medium">
                {topic.title}
              </p>
              <div className="flex items-center gap-2 text-xs">
                <span className="text-[#FF4500]">{topic.posts}</span>
                <span className="text-zinc-600">•</span>
                <span className="text-zinc-600">{topic.type}</span>
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}