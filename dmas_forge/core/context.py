from contextvars import ContextVar

class Context:
    def __init__(self) -> None:
        self._ctx = ContextVar("context", default={})

    def get(self, key, default=None):
        return self._ctx.get().get(key, default)

    def with_value(self, key, value):
        ctx = dict(self._ctx.get())
        ctx[key] = value
        token = self._ctx.set(ctx)
        self.token = token

    def reset(self):
        self._ctx.reset(self.token)