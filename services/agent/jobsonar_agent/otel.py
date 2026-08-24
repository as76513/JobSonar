from opentelemetry import trace
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import ConsoleSpanExporter, SimpleSpanProcessor

from jobsonar_agent import config

_wired = False


def setup() -> None:
    global _wired
    if _wired:
        return
    provider = TracerProvider(resource=Resource.create({"service.name": "jobsonar-agent"}))
    if config.OTEL_CONSOLE:
        provider.add_span_processor(SimpleSpanProcessor(ConsoleSpanExporter()))
    trace.set_tracer_provider(provider)
    _wired = True


def tracer():
    setup()
    return trace.get_tracer("jobsonar.agent")
