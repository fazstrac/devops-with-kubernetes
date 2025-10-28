import { fetchTodos, addTodo } from '../todo-api';

describe('todo API helpers', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetchTodos returns parsed JSON', async () => {
    const mock = [{ uuid: '1', description: 'a', createdAt: '2025-01-01T00:00:00Z' }];
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => mock } as any);

    const res = await fetchTodos();
    expect(res).toEqual(mock);
    expect(globalThis.fetch).toHaveBeenCalledWith('/todos');
  });

  it('addTodo posts data and returns created todo', async () => {
    const created = { uuid: '2', description: 'b', createdAt: '2025-01-01T00:00:00Z' };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => created } as any);

    const res = await addTodo('b');
    expect(res).toEqual(created);
    expect(globalThis.fetch).toHaveBeenCalledWith('/todos', expect.objectContaining({ method: 'POST' }));
  });
});
