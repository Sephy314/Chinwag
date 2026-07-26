export const API_PATHS = {
  auth: {
    whoami: "/auth/whoami",
    login: "/auth/login",
    register: "/auth/user",
    user: (id: string) => `/auth/user/${id}`,
    google: "/auth/google",
    googleCallback: "/auth/google/callback",
    logout: "/auth/logout",
  },
  rooms: {
    list: (userId: string) => `/users/${userId}/rooms`,
    get: (id: string) => `/rooms/${id}`,
    create: "/rooms",
    update: (id: string) => `/rooms/${id}`,
    delete: (id: string) => `/rooms/${id}`,
    members: (roomId: string) => `/rooms/${roomId}/members`,
    member: (roomId: string, userId: string) => `/rooms/${roomId}/members/${userId}`,
    invite: (roomId: string) => `/rooms/${roomId}/invite`,
    joinInvite: (token: string) => `/rooms/invite/${token}/join`,
    pop: (roomId: string) => `/rooms/${roomId}/pop`,
  },
  chat: {
    messages: (roomId: string) => `/chat/rooms/${roomId}/messages`,
    message: (roomId: string, messageId: string) => `/chat/rooms/${roomId}/messages/${messageId}`,
  },
} as const
