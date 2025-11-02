/* todo-api.ts
 *
 * This file contains functions to interact with the Todo API.
 */

/* eslint-disable @typescript-eslint/no-explicit-any */


/* Type definition for a Todo item */
export type Todo = {
  uuid: string;
  description: string;
  created_at: string;
};

/* Fetch all todos
 *
 * @returns A promise that resolves to an array of todos
 */
export async function fetchTodos(): Promise<Todo[]> {
  const res = await fetch("/todos");
  return res.json();
}

/* Add a new todo
 *
 * @param description - The description of the todo
 * @returns A promise that resolves to the created todo
 */
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

/* Delete a todo by its UUID
 *
 * @param uuid - The UUID of the todo to delete
 * @returns A promise that resolves when the todo is deleted
 */
export async function deleteTodo(uuid: string): Promise<void> {
  await fetch(`/todos/${uuid}`, {
    method: "DELETE",
  });
}

/* Update a todo's description by its UUID
 *
 * @param uuid - The UUID of the todo to update
 * @param description - The new description of the todo
 * @returns A promise that resolves to the updated todo
 */
export async function updateTodo(uuid: string, description: string): Promise<Todo> {
  const res = await fetch(`/todos/${uuid}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ description }),
  });
  return res.json();
}
