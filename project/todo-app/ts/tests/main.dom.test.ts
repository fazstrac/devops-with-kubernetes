import { describe, it, beforeEach, vi, expect } from 'vitest';
import { screen } from '@testing-library/dom';
import { initTodoApp } from '../main';
import * as api from '../todo-api';

describe('main UI wiring', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    document.body.innerHTML = `
      <input id="todo-input" />
      <button id="todo-send">Add</button>
      <div id="char-counter"></div>
      <ul id="todo-list"></ul>
    `;
  });

  it('loads existing todos and adds a new one', async () => {
    const mockTodos = [{ uuid: '1', description: 'existing', createdAt: '2025-01-01T00:00:00Z' }];
    vi.spyOn(api, 'fetchTodos').mockResolvedValue(mockTodos as any);
    vi.spyOn(api, 'addTodo').mockResolvedValue({ uuid: '2', description: 'new', createdAt: '2025-01-01T00:00:00Z' } as any);

    initTodoApp();

    // existing todo is rendered (wait for async loading)
    await screen.findByText('existing');

    const input = screen.getByRole('textbox') as HTMLInputElement;
    input.value = 'new';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    const button = screen.getByText('Add');
    button.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    // new todo appended (wait for async append)
    await screen.findByText('new');
  });
});
