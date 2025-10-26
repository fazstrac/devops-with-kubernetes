import { fetchTodos, addTodo } from "./todo";

(() => {
  const input = document.getElementById('todo-input') as HTMLInputElement | null;
  const send = document.getElementById('todo-send') as HTMLButtonElement | null;
  const counter = document.getElementById('char-counter') as HTMLElement | null;
  const list = document.getElementById('todo-list') as HTMLUListElement | null;

  if (!input || !send || !counter || !list) {
    // Required elements are missing; fail fast in dev but avoid throwing in prod.
    // Consumers compiling this file can decide how to handle it.
    // eslint-disable-next-line no-console
    console.warn('todo frontend: missing required DOM elements');
    return;
  }

  // Narrow to non-null local variables so TypeScript can reason about nullability
  const inputEl = input as HTMLInputElement;
  const sendBtn = send as HTMLButtonElement;
  const counterEl = counter as HTMLElement;
  const listEl = list as HTMLUListElement;

  const MAX_LEN = 140;

  async function loadTodos(): Promise<void> {
    try {
      const todos = await fetchTodos();
      todos.forEach(todo => {
        const li = document.createElement('li');
        li.textContent = todo.description;
        listEl.appendChild(li);
      });
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('Failed to load todos:', err);
    }
  }

  // initial load
  loadTodos();

  function updateCounter(): void {
    const len = inputEl.value.length;
    counterEl.textContent = `${len} / ${MAX_LEN}`;
    if (len > MAX_LEN) {
      counterEl.style.color = 'red';
      sendBtn.disabled = true;
    } else {
      counterEl.style.color = '#666';
      sendBtn.disabled = false;
    }
  }

  // live counter
  inputEl.addEventListener('input', updateCounter);
  updateCounter();

  // add todo locally (no server call)
  sendBtn.addEventListener('click', async () => {
    const text = inputEl.value.trim();
    if (!text) return;
    if (text.length > MAX_LEN) {
      // keep browser native alert for simplicity in this demo app
      alert(`Todo too long (max ${MAX_LEN} chars).`);
      return;
    }

    try {
      const todo = await addTodo(text);
      const li = document.createElement('li');
      li.textContent = todo.description;
      listEl.appendChild(li);
      inputEl.value = '';
      updateCounter();
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('Failed to add todo:', err);
    }

    inputEl.focus();
  });

  // allow enter key to submit
  inputEl.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      sendBtn.click();
    }
  });
})();
