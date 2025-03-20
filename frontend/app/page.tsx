import type React from "react"
import { SearchInterface } from "@/app/components/search-interface"
import { TrendingTopics } from "@/app/components/trending-topics"
import { Inter } from "next/font/google"

const inter = Inter({ subsets: ["latin"] })

export default function Page() {
  return (
    <div className={`min-h-screen bg-[#1A1A1B] text-white ${inter.className} p-4`}>
      <main className="container max-w-3xl mx-auto pt-16">
        <div className="space-y-8">
          <div className="space-y-2 text-center">
            <div className="flex items-center justify-center gap-2 mb-6">
              <div className="w-10 h-10 rounded-full bg-[#FF4500] flex items-center justify-center">
                <span className="text-white font-bold text-xl">S</span>
              </div>
              <h1 className="text-2xl font-bold">Subplexity</h1>
            </div>
            <h2 className="text-4xl font-bold tracking-tight">What do you want to explore on Reddit?</h2>
            <p className="text-zinc-400">AI-powered search across all Reddit threads and posts</p>
          </div>
          <SearchInterface />
          <TrendingTopics />
        </div>
      </main>
      <footer className="fixed bottom-4 left-4 right-4">
        <div className="container max-w-3xl mx-auto flex items-center justify-between text-sm text-zinc-600">
          <span>© 2025 Subplexity. All rights reserved.</span>
          <div className="flex items-center gap-4">
            <button className="hover:text-zinc-400 transition-colors">
              <RedditIcon className="h-5 w-5" />
            </button>
          </div>
        </div>
      </footer>
    </div>
  )
}

function RedditIcon(props: React.ComponentProps<"svg">) {
  return (
    <svg {...props} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <g clipPath="url(#clip0_1234_1234)">
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M20 10c0 5.523-4.477 10-10 10S0 15.523 0 10 4.477 0 10 0s10 4.477 10 10zm-8.5-1a1.5 1.5 0 100-3 1.5 1.5 0 000 3zm-4.5 6c2.5 0 4.5-1.5 4.5-3s-2-3-4.5-3-4.5 1.5-4.5 3 2 3 4.5 3zm7.5-3c0 1.5 2 3 4.5 3s4.5-1.5 4.5-3-2-3-4.5-3-4.5 1.5-4.5 3z"
          fill="currentColor"
        />
      </g>
      <defs>
        <clipPath id="clip0_1234_1234">
          <path fill="currentColor" d="M0 0h20v20H0z" />
        </clipPath>
      </defs>
    </svg>
  )
}