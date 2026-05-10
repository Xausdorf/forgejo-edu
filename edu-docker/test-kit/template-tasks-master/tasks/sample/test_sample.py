from sample import add


def test_add_positive() -> None:
    assert add(2, 3) == 5


def test_add_negative() -> None:
    assert add(-1, -1) == -2


def test_add_zero() -> None:
    assert add(0, 0) == 0
