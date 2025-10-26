"use strict";
(() => {
  // ts/todo.ts
  async function fetchTodos() {
    const res = await fetch("/todos");
    return res.json();
  }
  async function addTodo(description) {
    const res = await fetch("/todos", {
      method: "POST",
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify({ description })
    });
    return res.json();
  }

  // ts/frontend.ts
  (() => {
    const input = document.getElementById("todo-input");
    const send = document.getElementById("todo-send");
    const counter = document.getElementById("char-counter");
    const list = document.getElementById("todo-list");
    if (!input || !send || !counter || !list) {
      console.warn("todo frontend: missing required DOM elements");
      return;
    }
    const inputEl = input;
    const sendBtn = send;
    const counterEl = counter;
    const listEl = list;
    const MAX_LEN = 140;
    async function loadTodos() {
      try {
        const todos = await fetchTodos();
        todos.forEach((todo) => {
          const li = document.createElement("li");
          li.textContent = todo.description;
          listEl.appendChild(li);
        });
      } catch (err) {
        console.error("Failed to load todos:", err);
      }
    }
    loadTodos();
    function updateCounter() {
      const len = inputEl.value.length;
      counterEl.textContent = `${len} / ${MAX_LEN}`;
      if (len > MAX_LEN) {
        counterEl.style.color = "red";
        sendBtn.disabled = true;
      } else {
        counterEl.style.color = "#666";
        sendBtn.disabled = false;
      }
    }
    inputEl.addEventListener("input", updateCounter);
    updateCounter();
    sendBtn.addEventListener("click", async () => {
      const text = inputEl.value.trim();
      if (!text) return;
      if (text.length > MAX_LEN) {
        alert(`Todo too long (max ${MAX_LEN} chars).`);
        return;
      }
      try {
        const todo = await addTodo(text);
        const li = document.createElement("li");
        li.textContent = todo.description;
        listEl.appendChild(li);
        inputEl.value = "";
        updateCounter();
      } catch (err) {
        console.error("Failed to add todo:", err);
      }
      inputEl.focus();
    });
    inputEl.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        sendBtn.click();
      }
    });
  })();
})();
