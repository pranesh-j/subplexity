// frontend/app/components/search-citations.tsx
import React from 'react';
import { ExternalLink } from 'lucide-react';
import { Citation } from './api-client';
import Link from 'next/link';

interface SearchCitationsProps {
  citations: Citation[];
}

export default function SearchCitations({ citations }: SearchCitationsProps) {
  if (!citations || citations.length === 0) {
    return null;
  }

  return (
    <div className="mt-6 border border-neutral-200 dark:border-neutral-800 rounded-xl overflow-hidden">
      <div className="bg-neutral-50 dark:bg-neutral-900 p-4 border-b border-neutral-200 dark:border-neutral-800">
        <h3 className="text-sm font-medium">Sources from Reddit</h3>
      </div>
      <div className="p-2">
        {citations.map((citation, index) => (
          <div 
            key={index} 
            className="p-2 rounded-lg hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
          >
            <div className="flex items-start gap-3">
              <div className="flex items-center justify-center bg-neutral-100 dark:bg-neutral-800 h-6 w-6 rounded-full flex-shrink-0">
                <span className="text-xs font-medium">{citation.index}</span>
              </div>
              <div className="flex-1 min-w-0">
                <Link
                  href={citation.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline line-clamp-1"
                >
                  {citation.title}
                  <ExternalLink className="inline-block ml-1 h-3 w-3" />
                </Link>
                <p className="text-xs text-neutral-500 dark:text-neutral-400">
                  r/{citation.subreddit} • {citation.type}
                </p>
                {citation.text && (
                  <p className="mt-1 text-xs text-neutral-700 dark:text-neutral-300 line-clamp-2">
                    "{citation.text}"
                  </p>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}