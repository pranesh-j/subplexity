// frontend/app/components/search-citations.tsx
import React from 'react';
import { ExternalLink, MessageSquare, ArrowUpCircle } from 'lucide-react';
import { Citation } from './api-client';
import Link from 'next/link';

interface SearchCitationsProps {
  citations: Citation[];
}

export default function SearchCitations({ citations }: SearchCitationsProps) {
  if (!citations || citations.length === 0) {
    return null;
  }

  // Helper function to get icon based on type
  const getTypeIcon = (type: string) => {
    switch(type) {
      case 'post':
        return <MessageSquare className="h-3 w-3" />;
      case 'comment':
        return <MessageSquare className="h-3 w-3" />;
      default:
        return <ArrowUpCircle className="h-3 w-3" />;
    }
  };

  return (
    <div className="mt-6 border border-neutral-200 dark:border-neutral-800 rounded-xl overflow-hidden bg-white dark:bg-neutral-900">
      <div className="bg-neutral-50 dark:bg-neutral-900 p-4 border-b border-neutral-200 dark:border-neutral-800">
        <h3 className="text-sm font-medium">Sources from Reddit</h3>
      </div>
      <div className="divide-y divide-neutral-200 dark:divide-neutral-800">
        {citations.map((citation, index) => (
          <div 
            key={index} 
            id={`citation-${citation.index}`}
            className="p-4 hover:bg-neutral-50 dark:hover:bg-neutral-800/50 transition-colors"
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
                  className="text-sm font-medium text-primary hover:underline line-clamp-1 flex items-center"
                >
                  {citation.title}
                  <ExternalLink className="inline-block ml-1 h-3 w-3 flex-shrink-0" />
                </Link>
                
                <div className="flex items-center gap-2 mt-1">
                  <span className="text-xs text-neutral-500 dark:text-neutral-400 flex items-center gap-1">
                    r/{citation.subreddit}
                  </span>
                  <span className="text-neutral-300 dark:text-neutral-600">•</span>
                  <span className="text-xs text-neutral-500 dark:text-neutral-400 flex items-center gap-1">
                    {getTypeIcon(citation.type)}
                    <span>{citation.type}</span>
                  </span>
                </div>
                
                {citation.text && (
                  <div className="mt-2 p-2 bg-neutral-50 dark:bg-neutral-800 rounded-md border border-neutral-100 dark:border-neutral-700">
                    <p className="text-xs text-neutral-700 dark:text-neutral-300 italic">
                      "{citation.text}"
                    </p>
                  </div>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}