import { fetchTodos, addTodo, deleteTodo, updateTodo } from '../todo-api';

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

  it('deleteTodo sends DELETE request', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as any);

    const todoId = '3';
    await deleteTodo(todoId);
    expect(globalThis.fetch).toHaveBeenCalledWith(`/todos/${todoId}`, expect.objectContaining({ method: 'DELETE' }));
  });

  it('updateTodo sends PUT request and returns updated todo', async () => {
    const updated = { uuid: '4', description: 'c updated', createdAt: '2025-01-01T00:00:00Z' };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => updated } as any);

    const res = await updateTodo('4', 'c updated');
    expect(res).toEqual(updated);
    expect(globalThis.fetch).toHaveBeenCalledWith('/todos/4', expect.objectContaining({ method: 'PUT' }));
  });
});
