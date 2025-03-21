// frontend/app/components/api-client.ts
export interface SearchRequest {
    query: string;
    searchMode: string;
    modelName: string;
    limit?: number;
  }
  
  export interface SearchResult {
    id: string;
    title: string;
    subreddit: string;
    author: string;
    content: string;
    url: string;
    createdUtc: number;
    score: number;
    commentCount?: number;
    type: string; // "post", "comment", or "subreddit"
    highlights?: string[]; // New field for key excerpts
  }
  
  export interface Citation {
    index: number;
    text: string;
    url: string;
    title: string;
    type: string;
    subreddit: string;
  }
  
  export interface RequestParams {
    query: string;
    searchMode: string;
    modelName: string;
    limit: number;
  }
  
  export interface SearchResponse {
    results: SearchResult[];
    totalCount: number;
    reasoning?: string;
    answer?: string;
    citations?: Citation[]; // New field for citations
    elapsedTime: number;
    lastUpdated?: number; // New timestamp field
    requestParams?: RequestParams; // New field for request metadata
  }
  
  export const searchReddit = async (request: SearchRequest): Promise<SearchResponse> => {
    const response = await fetch('http://localhost:8080/api/search', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    });
  
    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to search Reddit');
    }
  
    return response.json();
  };