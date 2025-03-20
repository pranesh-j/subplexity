"use client"

import { useState } from "react"
import { Globe, Code, Image, Video } from "lucide-react"
import { Button } from "./ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./ui/select"
import { TextareaAutosize } from "./ui/textarea-autosize"
import { searchReddit, SearchResponse } from "./api-client"

const searchModes = [
  { icon: Globe, label: "All" },
  { icon: Code, label: "Posts" },
  { icon: Image, label: "Comments" },
  { icon: Video, label: "Communities" },
]

export function SearchInterface() {
  const [query, setQuery] = useState("")
  const [searchMode, setSearchMode] = useState("All")
  const [modelName, setModelName] = useState("DeepSeek R1")
  const [isSearching, setIsSearching] = useState(false)
  const [searchResults, setSearchResults] = useState<SearchResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleSearch = async () => {
    if (!query.trim()) {
      setError("Please enter a search query")
      return
    }

    setIsSearching(true)
    setError(null)
    
    try {
      const results = await searchReddit({
        query,
        searchMode,
        modelName,
      })
      
      setSearchResults(results)
    } catch (err) {
      console.error("Search error:", err)
      setError("Failed to search Reddit: " + (err instanceof Error ? err.message : "Unknown error"))
    } finally {
      setIsSearching(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="relative">
        <div className="relative rounded-xl border-2 border-zinc-800 bg-zinc-900/80 backdrop-blur-xl focus-within:border-[#FF4500] transition-all duration-200">
          <div className="flex items-center px-4 py-3">
            <TextareaAutosize
              className="flex-1 px-0 py-2 bg-transparent border-none text-lg placeholder:text-zinc-600 focus:outline-none resize-none overflow-y-auto scrollbar scrollbar-thumb-zinc-600 scrollbar-track-transparent leading-relaxed"
              placeholder="Ask anything Reddit..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              minRows={1}
              maxRows={8}
              style={{
                scrollbarWidth: "thin",
                scrollbarColor: "#52525b transparent",
              }}
            />
            <Button
              size="sm"
              className="ml-3 bg-[#FF4500] hover:bg-[#FF4500]/90 text-white px-6 flex-shrink-0 rounded-lg"
              onClick={handleSearch}
              disabled={isSearching}
            >
              {isSearching ? "Searching..." : "Search"}
            </Button>
          </div>
          <div className="flex items-center justify-between gap-1 px-4 py-2 border-t border-zinc-800">
            <div className="flex items-center gap-1">
              {searchModes.map((mode) => (
                <Button
                  key={mode.label}
                  variant="ghost"
                  size="sm"
                  className={`text-zinc-400 hover:text-white hover:bg-zinc-800 ${
                    searchMode === mode.label ? "bg-zinc-800 text-white" : ""
                  }`}
                  onClick={() => setSearchMode(mode.label)}
                >
                  <mode.icon className="h-4 w-4 mr-2" />
                  {mode.label}
                </Button>
              ))}
            </div>
            <Select 
              defaultValue="DeepSeek R1"
              value={modelName}
              onValueChange={setModelName}
            >
              <SelectTrigger className="w-[130px] border-zinc-700 bg-zinc-800/50 text-sm flex-shrink-0 hover:bg-zinc-800 hover:text-white transition-colors">
                <SelectValue placeholder="Select Model" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="Google Gemini">Google Gemini</SelectItem>
                <SelectItem value="Claude">Claude</SelectItem>
                <SelectItem value="DeepSeek R1">DeepSeek R1</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-900/20 border border-red-800 rounded-lg text-red-400">
          {error}
        </div>
      )}

      {searchResults && (
        <div className="space-y-6 mt-8">
          <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-6">
            <h2 className="text-xl font-bold mb-4">AI Analysis</h2>
            {searchResults.reasoning && (
              <div className="mb-6">
                <h3 className="text-sm font-medium text-zinc-400 mb-2">Reasoning</h3>
                <div className="p-4 bg-zinc-800/50 rounded-lg text-zinc-300">
                  {searchResults.reasoning}
                </div>
              </div>
            )}
            {searchResults.answer && (
              <div>
                <h3 className="text-sm font-medium text-zinc-400 mb-2">Answer</h3>
                <div className="text-white leading-relaxed">
                  {searchResults.answer}
                </div>
              </div>
            )}
          </div>

          <div>
            <h2 className="text-xl font-bold mb-4">Search Results</h2>
            <div className="space-y-4">
              {searchResults.results.map((result) => (
                <div 
                  key={result.id} 
                  className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4 hover:border-zinc-700 transition-colors"
                >
                  <div className="flex items-start gap-3">
                    <div className="bg-[#FF4500]/10 text-[#FF4500] rounded-full p-2 flex-shrink-0">
                      {result.type === "post" && <Code className="h-4 w-4" />}
                      {result.type === "comment" && <Image className="h-4 w-4" />}
                      {result.type === "subreddit" && <Video className="h-4 w-4" />}
                    </div>
                    <div className="flex-1">
                      <h3 className="text-lg font-medium">
                        <a 
                          href={result.url} 
                          target="_blank" 
                          rel="noopener noreferrer" 
                          className="hover:text-[#FF4500] transition-colors"
                        >
                          {result.title}
                        </a>
                      </h3>
                      <div className="flex items-center gap-2 text-sm text-zinc-400 mt-1">
                        <span>r/{result.subreddit}</span>
                        <span>•</span>
                        <span>u/{result.author}</span>
                        <span>•</span>
                        <span>{new Date(result.createdUtc * 1000).toLocaleDateString()}</span>
                      </div>
                      {result.content && (
                        <p className="mt-3 text-zinc-300 line-clamp-3">
                          {result.content}
                        </p>
                      )}
                      <div className="flex items-center gap-4 mt-3 text-sm">
                        <span className="text-zinc-400">
                          {result.score} points
                        </span>
                        {result.commentCount !== undefined && (
                          <span className="text-zinc-400">
                            {result.commentCount} comments
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}