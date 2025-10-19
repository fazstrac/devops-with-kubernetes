(function () {
    const input = document.getElementById('todo-input');
    const send = document.getElementById('todo-send');
    const counter = document.getElementById('char-counter');
    const list = document.getElementById('todo-list');

    function updateCounter() {
    const len = input.value.length;
    counter.textContent = `${len} / 140`;
    if (len > 140) {
        counter.style.color = 'red';
        send.disabled = true;
    } else {
        counter.style.color = '#666';
        send.disabled = false;
    }
    }

    // live counter
    input.addEventListener('input', updateCounter);
    updateCounter();

    // add todo locally (no server call)
    send.addEventListener('click', function () {
    const text = input.value.trim();
    if (!text) return;
    if (text.length > 140) {
        alert('Todo too long (max 140 chars).');
        return;
    }
    const li = document.createElement('li');
    li.textContent = text;
    list.appendChild(li);
    input.value = '';
    updateCounter();
    input.focus();
    });

    // allow enter key to submit
    input.addEventListener('keydown', function (e) {
    if (e.key === 'Enter') {
        e.preventDefault();
        send.click();
    }
    });
})();
