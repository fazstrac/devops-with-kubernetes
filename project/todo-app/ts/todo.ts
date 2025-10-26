export type Todo = {
  uuid: string;
  description: string;
  createdAt: string;
};

export async function fetchTodos(): Promise<Todo[]> {
  const res = await fetch("/todos");
  return res.json();
}

export async function addTodo(description: string): Promise<Todo> {
  const res = await fetch("/todos", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ description }),
  });
  return res.json();
}

