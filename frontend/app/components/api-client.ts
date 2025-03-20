// api-client.ts
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
  }
  
  export interface SearchResponse {
    results: SearchResult[];
    totalCount: number;
    reasoning?: string;
    answer?: string;
    elapsedTime: number;
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